package progress

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// captureLog sets up slog to write to a buffer and returns the buffer.
// Call the returned restore function to restore the default logger.
func captureLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	return &buf, func() { slog.SetDefault(oldLogger) }
}

func TestNewReporter(t *testing.T) {
	r := NewReporter(50)
	if r.totalFiles != 50 {
		t.Errorf("expected totalFiles=50, got %d", r.totalFiles)
	}
	if r.processedFiles != 0 || r.uploadedFiles != 0 || r.bytesUploaded != 0 || r.lastPercent != 0 {
		t.Error("expected all counters to be zero")
	}
}

func TestReport_CountersUpdate(t *testing.T) {
	r := NewReporter(10)

	// Report an uploaded file
	r.Report(true, 1024)
	if r.processedFiles != 1 {
		t.Errorf("expected processedFiles=1, got %d", r.processedFiles)
	}
	if r.uploadedFiles != 1 {
		t.Errorf("expected uploadedFiles=1, got %d", r.uploadedFiles)
	}
	if r.bytesUploaded != 1024 {
		t.Errorf("expected bytesUploaded=1024, got %d", r.bytesUploaded)
	}

	// Report a non-uploaded file (e.g., skipped due to dedup)
	r.Report(false, 0)
	if r.processedFiles != 2 {
		t.Errorf("expected processedFiles=2, got %d", r.processedFiles)
	}
	if r.uploadedFiles != 1 {
		t.Errorf("expected uploadedFiles=1 (unchanged), got %d", r.uploadedFiles)
	}
	if r.bytesUploaded != 1024 {
		t.Errorf("expected bytesUploaded=1024 (unchanged), got %d", r.bytesUploaded)
	}
}

func TestReport_LessThan100_LogsOnPercentChange(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	// 5 files total: each file is 20%, so percent changes at each file
	// file 1: 20%, file 2: 40%, file 3: 60%, file 4: 80%, file 5: 100%
	// All cross a percent boundary → 5 logs
	r := NewReporter(5)

	for i := 0; i < 5; i++ {
		r.Report(true, 100)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 log lines (percent changes at each file), got %d", len(lines))
	}
}

func TestReport_GreaterOrEqual100_LogsOnPercentBoundary(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	r := NewReporter(200)

	// Process 2 files = 1% of 200 → should log at file 2 (1%)
	r.Report(true, 50) // file 1: 0% → no log (0 > 0 is false)
	r.Report(true, 50) // file 2: 1% → log

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 log line at 1%% boundary, got %d: %s", len(lines), buf.String())
	}
}

func TestReport_GreaterOrEqual100_NoLogWithinSamePercent(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	r := NewReporter(1000)

	// Process 10 files = 1% of 1000 → log at file 10 (1% boundary)
	for i := 0; i < 10; i++ {
		r.Report(true, 100)
	}

	// Should have exactly 1 log line (at 1%)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 log line, got %d", len(lines))
	}

	// Process 10 more files = 2%
	buf.Reset()
	for i := 0; i < 10; i++ {
		r.Report(false, 0)
	}

	lines = strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 log line at 2%%, got %d", len(lines))
	}
}

func TestReport_LogsEvery100Files(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	// 50000 files: 1% = 500 files. But every 100 files also triggers.
	// So in the first 500 files, we get logs at: 100, 200, 300, 400, 500
	// file 100: 0% (100*100/50000=0), but 100%100==0 → log
	// file 200: 0%, but 200%100==0 → log
	// file 500: 1% boundary → log
	r := NewReporter(50000)

	for i := 0; i < 500; i++ {
		r.Report(true, 10)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// Logs at file 100, 200, 300, 400, 500 = 5 lines
	if len(lines) != 5 {
		t.Errorf("expected 5 log lines (every 100 files), got %d", len(lines))
	}
}

func TestReport_ZeroTotalFiles_NoPanic(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	r := NewReporter(0)
	r.Report(true, 100) // should not panic or log

	if buf.Len() != 0 {
		t.Errorf("expected no log output for zero total files, got: %s", buf.String())
	}
}

func TestReport_LogContainsExpectedFields(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	r := NewReporter(10)
	r.Report(true, 2048)

	output := buf.String()
	// Check that the log contains expected fields
	if !strings.Contains(output, "percent=") {
		t.Error("log should contain 'percent' field")
	}
	if !strings.Contains(output, "processed=") {
		t.Error("log should contain 'processed' field")
	}
	if !strings.Contains(output, "uploaded=") {
		t.Error("log should contain 'uploaded' field")
	}
	if !strings.Contains(output, "uploadedSize=") {
		t.Error("log should contain 'uploadedSize' field")
	}
}

func TestFormatSize_Bytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
	}
	for _, tc := range tests {
		result := FormatSize(tc.input)
		if result != tc.expected {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestFormatSize_KB(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024*1024 - 1, "1024.00 KB"},
	}
	for _, tc := range tests {
		result := FormatSize(tc.input)
		if result != tc.expected {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestFormatSize_MB(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 500, "500.00 MB"},
		{1024*1024*1024 - 1, "1024.00 MB"},
	}
	for _, tc := range tests {
		result := FormatSize(tc.input)
		if result != tc.expected {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestFormatSize_GB(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 5, "5.00 GB"},
		{1024 * 1024 * 1024 * 100, "100.00 GB"},
	}
	for _, tc := range tests {
		result := FormatSize(tc.input)
		if result != tc.expected {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
