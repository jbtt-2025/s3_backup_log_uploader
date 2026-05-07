package uploader

import (
	"context"
	"errors"
	"log-uploader/internal/interfaces"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// retryBackoffs 定义重试退避间隔
var retryBackoffs = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// maxAttempts 定义最大尝试次数（含首次）
const maxAttempts = 4

// Client 封装 S3 客户端操作
type Client struct {
	s3Client interfaces.S3API
	bucket   string
	sleepFn  func(time.Duration) // 用于测试时替换 time.Sleep
}

// NewClient 创建一个新的上传客户端。
// 使用静态凭证、自定义端点和 "auto" 区域（CloudFlare R2 约定）。
func NewClient(ak, sk, endpoint, bucket string) (*Client, error) {
	s3Client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(ak, sk, ""),
		),
		UsePathStyle: true,
	})

	return &Client{
		s3Client: s3Client,
		bucket:   bucket,
		sleepFn:  time.Sleep,
	}, nil
}

// NewClientWithAPI 创建一个使用自定义 S3API 实现的客户端（用于测试）。
func NewClientWithAPI(api interfaces.S3API, bucket string) *Client {
	return &Client{
		s3Client: api,
		bucket:   bucket,
		sleepFn:  time.Sleep,
	}
}

// Exists 通过 HeadObject 检查对象是否已存在。
// 包含重试逻辑：最多 4 次尝试（含首次），指数退避 1s/2s/4s。
// 返回 (true, nil) 表示已存在，(false, nil) 表示不存在，
// (false, err) 表示所有重试均失败。
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: &c.bucket,
			Key:    &key,
		})

		if err == nil {
			// 对象存在
			return true, nil
		}

		// 检查是否为 404 错误
		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 404 {
			// 对象不存在，这是正常情况
			return false, nil
		}

		// 其他错误（网络错误、5xx 等），需要重试
		lastErr = err

		// 如果还有重试机会，等待退避时间
		if attempt < maxAttempts-1 {
			c.sleepFn(retryBackoffs[attempt])
		}
	}

	// 所有重试均失败
	return false, lastErr
}

// Upload 上传文件到指定 key。
// 包含重试逻辑：最多 4 次尝试（含首次），指数退避 1s/2s/4s。
// 每次上传操作超时 30 秒。
func (c *Client) Upload(ctx context.Context, key string, filePath string) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 每次尝试重新打开文件（重置读取位置）
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}

		// 创建带 30 秒超时的上下文
		uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		_, putErr := c.s3Client.PutObject(uploadCtx, &s3.PutObjectInput{
			Bucket: &c.bucket,
			Key:    &key,
			Body:   file,
		})

		file.Close()
		cancel()

		if putErr == nil {
			return nil
		}

		lastErr = putErr

		// 如果还有重试机会，等待退避时间
		if attempt < maxAttempts-1 {
			c.sleepFn(retryBackoffs[attempt])
		}
	}

	return lastErr
}
