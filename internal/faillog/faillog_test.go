package faillog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWriter_DoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	w := NewWriterWithPath(path)
	defer w.Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected file to not exist before any Record call")
	}
}

func TestWriter_Record_CreatesFileOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	w := NewWriterWithPath(path)
	defer w.Close()

	err := w.Record("logs/host1/nginx/access.log.tar.gz")
	if err != nil {
		t.Fatalf("Record() returned error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected file to exist after Record call")
	}
}

func TestWriter_Record_WritesOneLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	w := NewWriterWithPath(path)

	key := "logs/host1/nginx/access.log.tar.gz"
	if err := w.Record(key); err != nil {
		t.Fatalf("Record() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := key + "\n"
	if string(data) != expected {
		t.Fatalf("expected %q, got %q", expected, string(data))
	}
}

func TestWriter_Record_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	w := NewWriterWithPath(path)

	keys := []string{
		"logs/host1/nginx/access.log.tar.gz",
		"logs/host1/nginx/error.log.tar.gz",
		"logs/host1/app/app.log.tar.gz",
	}

	for _, key := range keys {
		if err := w.Record(key); err != nil {
			t.Fatalf("Record(%q) returned error: %v", key, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != len(keys) {
		t.Fatalf("expected %d lines, got %d", len(keys), len(lines))
	}
	for i, line := range lines {
		if line != keys[i] {
			t.Fatalf("line %d: expected %q, got %q", i, keys[i], line)
		}
	}
}

func TestWriter_TruncatesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	// Pre-create the file with old content
	oldContent := "old/path/1\nold/path/2\nold/path/3\n"
	if err := os.WriteFile(path, []byte(oldContent), 0644); err != nil {
		t.Fatalf("failed to write pre-existing file: %v", err)
	}

	w := NewWriterWithPath(path)
	if err := w.Record("new/path/only"); err != nil {
		t.Fatalf("Record() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "new/path/only\n"
	if string(data) != expected {
		t.Fatalf("expected file to be truncated with new content %q, got %q", expected, string(data))
	}
}

func TestWriter_NoFailures_NoFileCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	w := NewWriterWithPath(path)
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected file to not exist when no failures recorded")
	}
}

func TestWriter_HasFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.log")

	w := NewWriterWithPath(path)
	defer w.Close()

	if w.HasFailures() {
		t.Fatal("expected HasFailures() to be false before any Record call")
	}

	if err := w.Record("some/key"); err != nil {
		t.Fatalf("Record() returned error: %v", err)
	}

	if !w.HasFailures() {
		t.Fatal("expected HasFailures() to be true after Record call")
	}
}

func TestNewWriter_UsesDefaultPath(t *testing.T) {
	w := NewWriter()
	if w.path != FailLogPath {
		t.Fatalf("expected default path %q, got %q", FailLogPath, w.path)
	}
}

func TestWriter_Record_InvalidPath_ReturnsError(t *testing.T) {
	// Use a path that cannot be created (directory doesn't exist)
	path := filepath.Join(t.TempDir(), "nonexistent", "subdir", "fail.log")

	w := NewWriterWithPath(path)
	defer w.Close()

	err := w.Record("some/key")
	if err == nil {
		t.Fatal("expected error when writing to invalid path")
	}
}
