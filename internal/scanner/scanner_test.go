package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper: 创建临时文件并设置修改时间
func createFile(t *testing.T, path string, content string, modTime time.Time) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestScan_BasicFiles(t *testing.T) {
	dir := t.TempDir()

	now := time.Now().Truncate(time.Second)
	earlier := now.Add(-1 * time.Hour)

	createFile(t, filepath.Join(dir, "a.log"), "hello", earlier)
	createFile(t, filepath.Join(dir, "b.log"), "world", now)

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// 按 ModTime 升序排列，earlier 的文件在前
	if filepath.Base(entries[0].AbsPath) != "a.log" {
		t.Errorf("expected first entry to be a.log, got %s", filepath.Base(entries[0].AbsPath))
	}
	if filepath.Base(entries[1].AbsPath) != "b.log" {
		t.Errorf("expected second entry to be b.log, got %s", filepath.Base(entries[1].AbsPath))
	}
}

func TestScan_SkipsZeroByteFiles(t *testing.T) {
	dir := t.TempDir()

	now := time.Now()
	createFile(t, filepath.Join(dir, "empty.log"), "", now)
	createFile(t, filepath.Join(dir, "notempty.log"), "data", now)

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skipping empty file), got %d", len(entries))
	}
	if filepath.Base(entries[0].AbsPath) != "notempty.log" {
		t.Errorf("expected notempty.log, got %s", filepath.Base(entries[0].AbsPath))
	}
}

func TestScan_IgnoreBeforeFilter(t *testing.T) {
	dir := t.TempDir()

	now := time.Now().Truncate(time.Second)
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	createFile(t, filepath.Join(dir, "old.log"), "old data", old)
	createFile(t, filepath.Join(dir, "recent.log"), "recent data", recent)

	// ignoreBefore 设为 24 小时前，应跳过 old.log
	ignoreBefore := now.Add(-24 * time.Hour)
	entries, err := Scan(dir, ignoreBefore)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if filepath.Base(entries[0].AbsPath) != "recent.log" {
		t.Errorf("expected recent.log, got %s", filepath.Base(entries[0].AbsPath))
	}
}

func TestScan_RecursiveSubdirectories(t *testing.T) {
	dir := t.TempDir()

	now := time.Now().Truncate(time.Second)

	createFile(t, filepath.Join(dir, "root.log"), "root", now)
	createFile(t, filepath.Join(dir, "sub1", "sub1.log"), "sub1", now.Add(-time.Minute))
	createFile(t, filepath.Join(dir, "sub1", "sub2", "deep.log"), "deep", now.Add(-2*time.Minute))

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// 验证 RelPath
	for _, e := range entries {
		switch filepath.Base(e.AbsPath) {
		case "root.log":
			if e.RelPath != "" {
				t.Errorf("root.log RelPath should be empty, got %q", e.RelPath)
			}
		case "sub1.log":
			if e.RelPath != "sub1" {
				t.Errorf("sub1.log RelPath should be 'sub1', got %q", e.RelPath)
			}
		case "deep.log":
			if e.RelPath != "sub1/sub2" {
				t.Errorf("deep.log RelPath should be 'sub1/sub2', got %q", e.RelPath)
			}
		}
	}
}

func TestScan_DirName(t *testing.T) {
	dir := t.TempDir()
	// TempDir 的最后一级目录名
	expectedDirName := filepath.Base(dir)

	now := time.Now()
	createFile(t, filepath.Join(dir, "test.log"), "data", now)

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].DirName != expectedDirName {
		t.Errorf("expected DirName %q, got %q", expectedDirName, entries[0].DirName)
	}
}

func TestScan_IsCompressed(t *testing.T) {
	dir := t.TempDir()

	now := time.Now()
	createFile(t, filepath.Join(dir, "app.log"), "data", now)
	createFile(t, filepath.Join(dir, "archive.zip"), "data", now)
	createFile(t, filepath.Join(dir, "backup.tar"), "data", now)
	createFile(t, filepath.Join(dir, "logs.tar.gz"), "data", now)
	createFile(t, filepath.Join(dir, "data.gz"), "data", now)

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	compressed := make(map[string]bool)
	for _, e := range entries {
		compressed[filepath.Base(e.AbsPath)] = e.IsCompressed
	}

	if compressed["app.log"] {
		t.Error("app.log should not be compressed")
	}
	if !compressed["archive.zip"] {
		t.Error("archive.zip should be compressed")
	}
	if !compressed["backup.tar"] {
		t.Error("backup.tar should be compressed")
	}
	if !compressed["logs.tar.gz"] {
		t.Error("logs.tar.gz should be compressed")
	}
	if !compressed["data.gz"] {
		t.Error("data.gz should be compressed")
	}
}

func TestScan_SortingByModTimeThenAbsPath(t *testing.T) {
	dir := t.TempDir()

	sameTime := time.Now().Truncate(time.Second)

	// 创建两个同一时间的文件，文件名字典序 b < c
	createFile(t, filepath.Join(dir, "c.log"), "c", sameTime)
	createFile(t, filepath.Join(dir, "b.log"), "b", sameTime)
	createFile(t, filepath.Join(dir, "a.log"), "a", sameTime.Add(-time.Hour))

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// a.log 最早，应排第一
	if filepath.Base(entries[0].AbsPath) != "a.log" {
		t.Errorf("expected first entry a.log, got %s", filepath.Base(entries[0].AbsPath))
	}
	// b.log 和 c.log 同时间，按 AbsPath 字典序
	if filepath.Base(entries[1].AbsPath) != "b.log" {
		t.Errorf("expected second entry b.log, got %s", filepath.Base(entries[1].AbsPath))
	}
	if filepath.Base(entries[2].AbsPath) != "c.log" {
		t.Errorf("expected third entry c.log, got %s", filepath.Base(entries[2].AbsPath))
	}
}

func TestScan_NonexistentDirectory(t *testing.T) {
	_, err := Scan("/nonexistent/path/that/does/not/exist", time.Time{})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestScan_UnreadableSubdirectory(t *testing.T) {
	// 此测试仅在非 Windows 系统上有效（Windows 权限模型不同）
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("skipping on Windows: permission model differs")
	}

	dir := t.TempDir()
	now := time.Now()

	createFile(t, filepath.Join(dir, "readable.log"), "data", now)
	subDir := filepath.Join(dir, "noperm")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	createFile(t, filepath.Join(subDir, "hidden.log"), "hidden", now)

	// 移除子目录的读权限
	if err := os.Chmod(subDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(subDir, 0755) // 恢复权限以便清理

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	// 应该只有 readable.log
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skipping unreadable subdir), got %d", len(entries))
	}
	if filepath.Base(entries[0].AbsPath) != "readable.log" {
		t.Errorf("expected readable.log, got %s", filepath.Base(entries[0].AbsPath))
	}
}

func TestScan_SkipsSymlinks(t *testing.T) {
	// 此测试仅在非 Windows 系统上有效
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("skipping on Windows: symlink creation requires elevated privileges")
	}

	dir := t.TempDir()
	now := time.Now()

	createFile(t, filepath.Join(dir, "real.log"), "data", now)

	// 创建符号链接
	symlink := filepath.Join(dir, "link.log")
	if err := os.Symlink(filepath.Join(dir, "real.log"), symlink); err != nil {
		t.Skip("cannot create symlink:", err)
	}

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	// 应该只有 real.log，符号链接被跳过
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skipping symlink), got %d", len(entries))
	}
	if filepath.Base(entries[0].AbsPath) != "real.log" {
		t.Errorf("expected real.log, got %s", filepath.Base(entries[0].AbsPath))
	}
}

func TestIsCompressedFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"app.log", false},
		{"data.txt", false},
		{"archive.zip", true},
		{"backup.tar", true},
		{"logs.tar.gz", true},
		{"data.gz", true},
		{"file.ZIP", false},  // 大写不匹配（区分大小写）
		{"file.TAR", false},  // 大写不匹配
		{"file.GZ", false},   // 大写不匹配
		{"file.tar.GZ", false}, // 部分大写不匹配
		{"noext", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := IsCompressedFile(tt.filename)
			if got != tt.expected {
				t.Errorf("IsCompressedFile(%q) = %v, want %v", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for empty directory, got %d", len(entries))
	}
}

func TestScan_FileSize(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	content := "hello world 12345"
	createFile(t, filepath.Join(dir, "sized.log"), content, now)

	entries, err := Scan(dir, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), entries[0].Size)
	}
}
