package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"log-uploader/internal/cleaner"
	"log-uploader/internal/compressor"
	"log-uploader/internal/config"
	"log-uploader/internal/faillog"
	"log-uploader/internal/interfaces"
	"log-uploader/internal/pathbuilder"
	"log-uploader/internal/progress"
	"log-uploader/internal/scanner"
	"log-uploader/internal/uploader"
)

// realEnvReader 实现 interfaces.EnvReader，从真实环境变量读取
type realEnvReader struct{}

func (r *realEnvReader) Getenv(key string) string { return os.Getenv(key) }

// realOSHost 实现 interfaces.OSHost，从操作系统获取主机名
type realOSHost struct{}

func (r *realOSHost) Hostname() (string, error) { return os.Hostname() }

func main() {
	os.Exit(run())
}

func run() int {
	// 1. 加载配置
	cfg, err := config.Load(&realEnvReader{}, &realOSHost{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置加载失败: %v\n", err)
		return 1
	}

	// 2. 创建 S3 上传客户端
	uploaderClient, err := uploader.NewClient(cfg.AK, cfg.SK, cfg.Endpoint, cfg.Bucket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建上传客户端失败: %v\n", err)
		return 1
	}

	// 3. 计算 ignoreBefore 时间
	var ignoreBefore time.Time
	if cfg.IgnoreDays > 0 {
		ignoreBefore = time.Now().AddDate(0, 0, -cfg.IgnoreDays)
	}

	// 4. 扫描所有日志目录
	var stats interfaces.Stats
	var allEntries []scanner.FileEntry
	atLeastOneDirSuccess := false

	for _, dir := range cfg.LogDirs {
		slog.Info("开始扫描目录", "dir", dir)
		entries, err := scanner.Scan(dir, ignoreBefore)
		if err != nil {
			slog.Warn("目录扫描失败，跳过", "dir", dir, "error", err)
			continue
		}
		atLeastOneDirSuccess = true
		stats.DirsProcessed++
		slog.Info("目录扫描完成", "dir", dir, "files", len(entries))
		allEntries = append(allEntries, entries...)
	}

	// 5. 如果所有目录都失败，退出
	if !atLeastOneDirSuccess {
		slog.Error("所有日志目录均不可访问")
		return 1
	}

	stats.TotalFiles = len(allEntries)

	// 6. 创建进度报告器
	reporter := progress.NewReporter(stats.TotalFiles)

	// 7. 创建失败日志写入器
	failWriter := faillog.NewWriter()
	defer failWriter.Close()

	// 8. 逐文件处理
	ctx := context.Background()

	for _, entry := range allEntries {
		processFile(ctx, cfg, uploaderClient, entry, &stats, reporter, failWriter)
	}

	// 9. 输出汇总日志
	slog.Info("处理完毕",
		"dirsProcessed", stats.DirsProcessed,
		"totalFiles", stats.TotalFiles,
		"filesUploaded", stats.FilesUploaded,
		"filesSkipped", stats.FilesSkipped,
		"filesFailed", stats.FilesFailed,
		"bytesUploaded", stats.BytesUploaded,
		"bytesUploadedHuman", progress.FormatSize(stats.BytesUploaded),
	)

	return 0
}

func processFile(
	ctx context.Context,
	cfg *config.Config,
	uploaderClient *uploader.Client,
	entry scanner.FileEntry,
	stats *interfaces.Stats,
	reporter *progress.Reporter,
	failWriter *faillog.Writer,
) {
	slog.Info("发现文件", "path", entry.AbsPath, "size", entry.Size, "compressed", entry.IsCompressed)

	var uploadFilePath string
	var uploadFileName string
	var uploadSize int64
	var needCleanup bool

	// a. 判断是否需要压缩
	if !entry.IsCompressed {
		slog.Info("压缩开始", "path", entry.AbsPath)
		result, err := compressor.Compress(entry.AbsPath)
		if err != nil {
			slog.Error("压缩失败", "path", entry.AbsPath, "error", err)
			stats.FilesFailed++
			stats.FilesProcessed++
			reporter.Report(false, 0)
			return
		}
		uploadFilePath = result.ArchivePath
		uploadFileName = filepath.Base(entry.AbsPath) + ".tar.gz"
		uploadSize = result.Size
		needCleanup = true
	} else {
		uploadFilePath = entry.AbsPath
		uploadFileName = filepath.Base(entry.AbsPath)
		uploadSize = entry.Size
		needCleanup = false
	}

	// b. 构造 S3 key
	key := pathbuilder.Build(cfg.BaseDir, cfg.Hostname, entry.DirName, entry.RelPath, uploadFileName)

	// c. 去重检查
	exists, err := uploaderClient.Exists(ctx, key)
	if err != nil {
		slog.Error("去重检查失败", "key", key, "error", err)
		stats.FilesFailed++
		stats.FilesProcessed++
		if err := failWriter.Record(key); err != nil {
			slog.Warn("记录失败日志出错", "error", err)
		}
		if needCleanup {
			cleanupArchive(uploadFilePath)
		}
		reporter.Report(false, 0)
		return
	}

	if exists {
		slog.Info("上传跳过（已存在）", "key", key)
		stats.FilesSkipped++
		stats.FilesProcessed++
		if needCleanup {
			cleanupArchive(uploadFilePath)
		}
		reporter.Report(false, 0)
		return
	}

	// d. 上传
	slog.Info("上传开始", "key", key, "size", uploadSize)
	err = uploaderClient.Upload(ctx, key, uploadFilePath)
	if err != nil {
		slog.Error("上传失败", "key", key, "error", err)
		stats.FilesFailed++
		stats.FilesProcessed++
		if err := failWriter.Record(key); err != nil {
			slog.Warn("记录失败日志出错", "error", err)
		}
		if needCleanup {
			cleanupArchive(uploadFilePath)
		}
		reporter.Report(false, 0)
		return
	}

	slog.Info("上传完成", "key", key, "size", uploadSize)
	stats.FilesUploaded++
	stats.BytesUploaded += uploadSize
	stats.FilesProcessed++

	// e. 清理临时归档
	if needCleanup {
		cleanupArchive(uploadFilePath)
	}

	// f. 报告进度
	reporter.Report(true, uploadSize)
}

func cleanupArchive(archivePath string) {
	slog.Info("清理临时归档", "path", archivePath)
	if err := cleaner.Cleanup(archivePath); err != nil {
		slog.Warn("清理临时归档失败", "path", archivePath, "error", err)
	}
}
