package scanner

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileEntry 表示一个待处理的文件
type FileEntry struct {
	AbsPath      string    // 文件绝对路径
	RelPath      string    // 相对于日志目录的路径（不含文件名）
	DirName      string    // 日志目录最后一级目录名
	ModTime      time.Time // 文件修改时间
	Size         int64     // 文件大小
	IsCompressed bool      // 是否为已压缩格式
}

// Scan 递归扫描指定目录，返回按修改时间升序排列的文件列表。
// 不可读的目录会被跳过并通过 slog 记录警告。
// ignoreBefore 为零值时不过滤；非零值时跳过 ModTime 早于该时间的文件。
func Scan(dir string, ignoreBefore time.Time) ([]FileEntry, error) {
	// 获取目录的绝对路径
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// 检查目录是否存在且可读
	info, err := os.Stat(absDir)
	if err != nil {
		slog.Warn("日志目录不可访问", "path", absDir, "error", err)
		return nil, err
	}
	if !info.IsDir() {
		slog.Warn("路径不是目录", "path", absDir)
		return nil, err
	}

	dirName := filepath.Base(absDir)
	var entries []FileEntry

	err = filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// 不可读的目录/子目录，记录警告并跳过
			slog.Warn("无法访问路径，跳过", "path", path, "error", err)
			return filepath.SkipDir
		}

		// 跳过目录本身（只处理文件）
		if d.IsDir() {
			return nil
		}

		// 跳过非常规文件（符号链接、设备文件、命名管道、套接字等）
		if !d.Type().IsRegular() {
			return nil
		}

		// 获取文件信息
		fileInfo, err := d.Info()
		if err != nil {
			slog.Warn("无法获取文件信息，跳过", "path", path, "error", err)
			return nil
		}

		// 跳过零字节文件
		if fileInfo.Size() == 0 {
			return nil
		}

		// 跳过修改时间早于 ignoreBefore 的文件（ignoreBefore 非零值时）
		if !ignoreBefore.IsZero() && fileInfo.ModTime().Before(ignoreBefore) {
			return nil
		}

		// 计算相对路径（不含文件名）
		absPath, err := filepath.Abs(path)
		if err != nil {
			slog.Warn("无法获取绝对路径，跳过", "path", path, "error", err)
			return nil
		}

		rel, err := filepath.Rel(absDir, absPath)
		if err != nil {
			slog.Warn("无法计算相对路径，跳过", "path", path, "error", err)
			return nil
		}

		// RelPath 是子目录部分（不含文件名）
		relPath := filepath.Dir(rel)
		if relPath == "." {
			relPath = ""
		}
		// 统一使用正斜杠
		relPath = filepath.ToSlash(relPath)

		entry := FileEntry{
			AbsPath:      absPath,
			RelPath:      relPath,
			DirName:      dirName,
			ModTime:      fileInfo.ModTime(),
			Size:         fileInfo.Size(),
			IsCompressed: IsCompressedFile(filepath.Base(path)),
		}

		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		// WalkDir 本身返回错误（根目录不可读等情况）
		return nil, err
	}

	// 按 ModTime 升序排序，相同时间按 AbsPath 字典序排序
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ModTime.Equal(entries[j].ModTime) {
			return entries[i].AbsPath < entries[j].AbsPath
		}
		return entries[i].ModTime.Before(entries[j].ModTime)
	})

	return entries, nil
}

// IsCompressedFile 根据文件名判断是否为已压缩格式（区分大小写）
// 已压缩扩展名：.zip、.tar、.tar.gz、.gz
func IsCompressedFile(filename string) bool {
	if strings.HasSuffix(filename, ".tar.gz") {
		return true
	}
	if strings.HasSuffix(filename, ".zip") {
		return true
	}
	if strings.HasSuffix(filename, ".tar") {
		return true
	}
	if strings.HasSuffix(filename, ".gz") {
		return true
	}
	return false
}
