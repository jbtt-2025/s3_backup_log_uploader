package cleaner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanup_DeletesFile(t *testing.T) {
	// Create a temp file to simulate a temporary archive
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "test.log.tar.gz")
	if err := os.WriteFile(tmpFile, []byte("fake archive content"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Verify the file exists before cleanup
	if _, err := os.Stat(tmpFile); err != nil {
		t.Fatalf("temp file should exist before cleanup: %v", err)
	}

	// Call Cleanup
	err := Cleanup(tmpFile)
	if err != nil {
		t.Fatalf("Cleanup returned unexpected error: %v", err)
	}

	// Verify the file no longer exists
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, but got err: %v", err)
	}
}

func TestCleanup_ReturnsErrorForNonexistentFile(t *testing.T) {
	err := Cleanup("/nonexistent/path/to/file.tar.gz")
	if err == nil {
		t.Fatal("expected error when deleting nonexistent file, got nil")
	}
}
