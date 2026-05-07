package cleaner

import "os"

// Cleanup 删除指定的临时归档文件。
// 如果删除失败，返回错误但不中断程序流程。
func Cleanup(archivePath string) error {
	return os.Remove(archivePath)
}
