package pathbuilder

import "strings"

// Build 构造上传路径。
// 格式: {baseDir}/{hostname}/{dirName}/{relPath}/{fileName}
// 如果 relPath 为空（文件在目录根下），则省略 relPath 部分。
// 始终使用正斜杠作为分隔符。
// 确保无连续斜杠或尾部斜杠。
func Build(baseDir, hostname, dirName, relPath, fileName string) string {
	parts := []string{baseDir, hostname, dirName}
	if relPath != "" {
		parts = append(parts, relPath)
	}
	parts = append(parts, fileName)

	// Join all parts with forward slash
	result := strings.Join(parts, "/")

	// Normalize: replace any backslashes with forward slashes
	result = strings.ReplaceAll(result, "\\", "/")

	// Remove consecutive slashes
	for strings.Contains(result, "//") {
		result = strings.ReplaceAll(result, "//", "/")
	}

	// Remove trailing slash
	result = strings.TrimRight(result, "/")

	return result
}
