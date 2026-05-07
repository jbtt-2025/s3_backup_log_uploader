package progress

import (
	"fmt"
	"log/slog"
)

// Reporter 跟踪处理进度并在达到阈值时输出进度信息
type Reporter struct {
	totalFiles     int
	processedFiles int
	uploadedFiles  int
	bytesUploaded  int64
	lastPercent    int // 上次输出时的百分比
}

// NewReporter 创建进度报告器。
// totalFiles 为待处理文件总数。
func NewReporter(totalFiles int) *Reporter {
	return &Reporter{
		totalFiles: totalFiles,
	}
}

// Report 在处理完一个文件后调用，更新统计并在达到 1% 步长时输出进度。
// uploaded 表示该文件是否成功上传，size 为上传的字节数。
// 如果总文件数 < 100，则每个文件都输出进度。
func (r *Reporter) Report(uploaded bool, size int64) {
	r.processedFiles++
	if uploaded {
		r.uploadedFiles++
		r.bytesUploaded += size
	}

	if r.totalFiles <= 0 {
		return
	}

	currentPercent := r.processedFiles * 100 / r.totalFiles

	if r.totalFiles < 100 {
		// 总文件数 < 100 时，每处理一个文件都输出进度
		r.logProgress(currentPercent)
		r.lastPercent = currentPercent
	} else {
		// 总文件数 >= 100 时，仅在百分比跨越 1% 边界时输出
		if currentPercent > r.lastPercent {
			r.logProgress(currentPercent)
			r.lastPercent = currentPercent
		}
	}
}

func (r *Reporter) logProgress(percent int) {
	slog.Info("progress",
		"percent", percent,
		"processed", r.processedFiles,
		"uploaded", r.uploadedFiles,
		"uploadedSize", FormatSize(r.bytesUploaded),
	)
}

// FormatSize 将字节数转换为人类可读格式。
// bytes < 1024 → "X B"
// < 1MB → "X.XX KB"
// < 1GB → "X.XX MB"
// else → "X.XX GB"
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%d B", bytes)
	case bytes < MB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	case bytes < GB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	default:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	}
}
