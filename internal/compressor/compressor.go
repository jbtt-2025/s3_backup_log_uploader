package compressor

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CompressResult 表示压缩操作的结果
type CompressResult struct {
	ArchivePath string // 生成的临时归档路径
	Size        int64  // 归档文件大小
}

// Compress 将指定文件压缩为 tar.gz 格式。
// 归档文件创建在原始文件所在目录，命名为 {原始文件名}.tar.gz。
// 如果目标路径已存在同名文件，将覆盖。
func Compress(filePath string) (*CompressResult, error) {
	// 打开源文件
	srcFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}

	// 构造目标归档路径: {sourceDir}/{sourceFileName}.tar.gz
	srcDir := filepath.Dir(filePath)
	srcName := filepath.Base(filePath)
	archivePath := filepath.Join(srcDir, srcName+".tar.gz")

	// 创建目标文件（如果已存在则覆盖）
	dstFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer func() {
		dstFile.Close()
	}()

	// 创建 gzip writer
	gzWriter := gzip.NewWriter(dstFile)

	// 创建 tar writer
	tarWriter := tar.NewWriter(gzWriter)

	// 写入 tar header（使用基础文件名，不含路径）
	header := &tar.Header{
		Name: srcName,
		Mode: int64(srcInfo.Mode().Perm()),
		Size: srcInfo.Size(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		// 清理：关闭 writers 和删除不完整的归档
		tarWriter.Close()
		gzWriter.Close()
		dstFile.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("failed to write tar header: %w", err)
	}

	// 复制文件内容到 tar writer
	if _, err := io.Copy(tarWriter, srcFile); err != nil {
		tarWriter.Close()
		gzWriter.Close()
		dstFile.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("failed to write file content to archive: %w", err)
	}

	// 按顺序关闭 writers
	if err := tarWriter.Close(); err != nil {
		gzWriter.Close()
		dstFile.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		dstFile.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	if err := dstFile.Close(); err != nil {
		os.Remove(archivePath)
		return nil, fmt.Errorf("failed to close archive file: %w", err)
	}

	// 获取归档文件大小
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat archive file: %w", err)
	}

	return &CompressResult{
		ArchivePath: archivePath,
		Size:        archiveInfo.Size(),
	}, nil
}
