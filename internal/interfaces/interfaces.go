package interfaces

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	// Ensure aws-sdk-go-v2 root module is a direct dependency
	_ "github.com/aws/aws-sdk-go-v2/aws"
)

// S3API 抽象 S3 操作，便于测试时 mock
type S3API interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// OSHost 抽象主机名获取
type OSHost interface {
	Hostname() (string, error)
}

// EnvReader 抽象环境变量读取
type EnvReader interface {
	Getenv(key string) string
}

// Stats 记录处理统计信息
type Stats struct {
	DirsProcessed  int   // 成功处理的目录数
	FilesUploaded  int   // 上传成功的文件数
	FilesSkipped   int   // 因去重跳过的文件数
	FilesFailed    int   // 处理失败的文件数
	BytesUploaded  int64 // 累计上传字节数
	TotalFiles     int   // 待处理文件总数
	FilesProcessed int   // 已处理文件数（含成功、跳过、失败）
}
