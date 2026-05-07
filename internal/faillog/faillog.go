package faillog

import (
	"fmt"
	"os"
)

// FailLogPath is the default path for the failure log file.
const FailLogPath = "/tmp/S3_backup_log_update_fail.log"

// Writer manages writing upload failure records to a log file.
// The file is lazily created only when the first failure is recorded.
type Writer struct {
	path        string
	hasFailures bool
	file        *os.File
}

// NewWriter creates a failure log writer using the default FailLogPath.
// The file is not created until the first Record() call.
func NewWriter() *Writer {
	return &Writer{path: FailLogPath}
}

// NewWriterWithPath creates a failure log writer using a custom path.
// This is useful for testing.
func NewWriterWithPath(path string) *Writer {
	return &Writer{path: path}
}

// Record records a failed S3 upload key to the failure log.
// On the first call, the file is created (or truncated if it already exists).
// Each call writes one line containing the s3Key.
func (w *Writer) Record(s3Key string) error {
	if w.file == nil {
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("faillog: failed to create log file %s: %w", w.path, err)
		}
		w.file = f
		w.hasFailures = true
	}

	_, err := fmt.Fprintf(w.file, "%s\n", s3Key)
	if err != nil {
		return fmt.Errorf("faillog: failed to write to log file: %w", err)
	}
	return nil
}

// Close closes the file handle if it was opened.
// If no failures were recorded, no file is created or left behind.
func (w *Writer) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// HasFailures returns whether any failures have been recorded.
func (w *Writer) HasFailures() bool {
	return w.hasFailures
}
