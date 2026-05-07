package compressor

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCompress_BasicFile(t *testing.T) {
	// 创建临时目录和测试文件
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.log")
	content := []byte("hello world\nthis is a test log file\n")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 执行压缩
	result, err := Compress(srcPath)
	if err != nil {
		t.Fatalf("Compress() returned error: %v", err)
	}

	// 验证归档路径正确
	expectedPath := filepath.Join(tmpDir, "test.log.tar.gz")
	if result.ArchivePath != expectedPath {
		t.Errorf("ArchivePath = %q, want %q", result.ArchivePath, expectedPath)
	}

	// 验证归档文件存在
	if _, err := os.Stat(result.ArchivePath); os.IsNotExist(err) {
		t.Fatal("archive file does not exist")
	}

	// 验证大小大于 0
	if result.Size <= 0 {
		t.Errorf("Size = %d, want > 0", result.Size)
	}
}

func TestCompress_ContentRoundTrip(t *testing.T) {
	// 创建临时目录和测试文件
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "data.txt")
	content := []byte("line1\nline2\nline3\nsome binary data: \x00\x01\x02\x03")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 执行压缩
	result, err := Compress(srcPath)
	if err != nil {
		t.Fatalf("Compress() returned error: %v", err)
	}

	// 解压并验证内容
	archiveFile, err := os.Open(result.ArchivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatalf("failed to read tar header: %v", err)
	}

	// 验证 tar 中的文件名是基础文件名
	if header.Name != "data.txt" {
		t.Errorf("tar entry name = %q, want %q", header.Name, "data.txt")
	}

	// 验证内容一致
	extracted, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatalf("failed to read tar content: %v", err)
	}

	if string(extracted) != string(content) {
		t.Errorf("extracted content does not match original.\ngot:  %q\nwant: %q", extracted, content)
	}

	// 验证只有一个条目
	_, err = tarReader.Next()
	if err != io.EOF {
		t.Error("expected only one entry in tar archive")
	}
}

func TestCompress_OverwriteExisting(t *testing.T) {
	// 创建临时目录和测试文件
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "overwrite.log")
	content1 := []byte("first content")
	if err := os.WriteFile(srcPath, content1, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 先创建一个同名的归档文件（模拟已存在）
	archivePath := filepath.Join(tmpDir, "overwrite.log.tar.gz")
	if err := os.WriteFile(archivePath, []byte("old archive data"), 0644); err != nil {
		t.Fatalf("failed to create existing archive: %v", err)
	}

	// 执行压缩（应覆盖已存在的文件）
	result, err := Compress(srcPath)
	if err != nil {
		t.Fatalf("Compress() returned error: %v", err)
	}

	if result.ArchivePath != archivePath {
		t.Errorf("ArchivePath = %q, want %q", result.ArchivePath, archivePath)
	}

	// 验证归档内容是新的（解压验证）
	archiveFile, err := os.Open(result.ArchivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	_, err = tarReader.Next()
	if err != nil {
		t.Fatalf("failed to read tar header: %v", err)
	}

	extracted, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatalf("failed to read tar content: %v", err)
	}

	if string(extracted) != string(content1) {
		t.Errorf("extracted content = %q, want %q (archive was not properly overwritten)", extracted, content1)
	}
}

func TestCompress_NonExistentFile(t *testing.T) {
	_, err := Compress("/nonexistent/path/file.log")
	if err == nil {
		t.Fatal("Compress() should return error for non-existent file")
	}
}

func TestCompress_EmptyFile(t *testing.T) {
	// 创建临时目录和空文件
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "empty.log")
	if err := os.WriteFile(srcPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 执行压缩
	result, err := Compress(srcPath)
	if err != nil {
		t.Fatalf("Compress() returned error: %v", err)
	}

	// 验证归档存在且大小 > 0（即使源文件为空，tar.gz 头部也有大小）
	if result.Size <= 0 {
		t.Errorf("Size = %d, want > 0 even for empty file", result.Size)
	}

	// 验证解压后内容为空
	archiveFile, err := os.Open(result.ArchivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatalf("failed to read tar header: %v", err)
	}

	if header.Name != "empty.log" {
		t.Errorf("tar entry name = %q, want %q", header.Name, "empty.log")
	}

	extracted, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatalf("failed to read tar content: %v", err)
	}

	if len(extracted) != 0 {
		t.Errorf("expected empty content, got %d bytes", len(extracted))
	}
}
