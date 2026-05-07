package uploader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log-uploader/internal/interfaces"
)

// mockS3API 是一个用于测试的 S3API mock 实现
type mockS3API struct {
	headObjectFunc func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	putObjectFunc  func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func (m *mockS3API) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headObjectFunc != nil {
		return m.headObjectFunc(ctx, params, optFns...)
	}
	return &s3.HeadObjectOutput{}, nil
}

func (m *mockS3API) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

// 确保 mockS3API 实现了 interfaces.S3API 接口
var _ interfaces.S3API = (*mockS3API)(nil)

func TestNewClient(t *testing.T) {
	client, err := NewClient("test-ak", "test-sk", "https://example.r2.cloudflarestorage.com", "test-bucket")
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
	if client.bucket != "test-bucket" {
		t.Errorf("expected bucket %q, got %q", "test-bucket", client.bucket)
	}
	if client.s3Client == nil {
		t.Error("expected s3Client to be non-nil")
	}
}

func TestNewClientWithAPI(t *testing.T) {
	mock := &mockS3API{}
	client := NewClientWithAPI(mock, "my-bucket")

	if client == nil {
		t.Fatal("NewClientWithAPI returned nil client")
	}
	if client.bucket != "my-bucket" {
		t.Errorf("expected bucket %q, got %q", "my-bucket", client.bucket)
	}
	if client.s3Client != mock {
		t.Error("expected s3Client to be the provided mock")
	}
}

func TestUpload_SuccessOnFirstAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	callCount := 0
	mock := &mockS3API{
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			callCount++
			if *params.Bucket != "test-bucket" {
				t.Errorf("expected bucket %q, got %q", "test-bucket", *params.Bucket)
			}
			if *params.Key != "logs/host/dir/test.log.tar.gz" {
				t.Errorf("expected key %q, got %q", "logs/host/dir/test.log.tar.gz", *params.Key)
			}
			return &s3.PutObjectOutput{}, nil
		},
	}

	client := NewClientWithAPI(mock, "test-bucket")
	client.sleepFn = func(d time.Duration) {
		t.Error("sleepFn should not be called on success")
	}

	err := client.Upload(context.Background(), "logs/host/dir/test.log.tar.gz", testFile)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 PutObject call, got %d", callCount)
	}
}

func TestUpload_RetryOnTransientErrorThenSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")
	if err := os.WriteFile(testFile, []byte("retry content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	callCount := 0
	transientErr := errors.New("network timeout")
	mock := &mockS3API{
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			callCount++
			if callCount <= 2 {
				return nil, transientErr
			}
			return &s3.PutObjectOutput{}, nil
		},
	}

	var sleepDurations []time.Duration
	client := NewClientWithAPI(mock, "test-bucket")
	client.sleepFn = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}

	err := client.Upload(context.Background(), "key", testFile)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 PutObject calls, got %d", callCount)
	}
	// 2 failures → 2 backoff sleeps: 1s, 2s
	if len(sleepDurations) != 2 {
		t.Fatalf("expected 2 sleep calls, got %d", len(sleepDurations))
	}
	if sleepDurations[0] != 1*time.Second {
		t.Errorf("expected first backoff 1s, got %v", sleepDurations[0])
	}
	if sleepDurations[1] != 2*time.Second {
		t.Errorf("expected second backoff 2s, got %v", sleepDurations[1])
	}
}

func TestUpload_AllRetriesExhaustedReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")
	if err := os.WriteFile(testFile, []byte("fail content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	callCount := 0
	persistentErr := errors.New("service unavailable")
	mock := &mockS3API{
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			callCount++
			return nil, persistentErr
		},
	}

	var sleepDurations []time.Duration
	client := NewClientWithAPI(mock, "test-bucket")
	client.sleepFn = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}

	err := client.Upload(context.Background(), "key", testFile)
	if err == nil {
		t.Fatal("Upload should have returned an error")
	}
	if err.Error() != "service unavailable" {
		t.Errorf("expected error %q, got %q", "service unavailable", err.Error())
	}
	if callCount != 4 {
		t.Errorf("expected 4 PutObject calls (maxAttempts), got %d", callCount)
	}
	// 3 backoff sleeps: 1s, 2s, 4s
	if len(sleepDurations) != 3 {
		t.Fatalf("expected 3 sleep calls, got %d", len(sleepDurations))
	}
	if sleepDurations[0] != 1*time.Second {
		t.Errorf("expected first backoff 1s, got %v", sleepDurations[0])
	}
	if sleepDurations[1] != 2*time.Second {
		t.Errorf("expected second backoff 2s, got %v", sleepDurations[1])
	}
	if sleepDurations[2] != 4*time.Second {
		t.Errorf("expected third backoff 4s, got %v", sleepDurations[2])
	}
}

func TestUpload_FileNotFoundReturnsImmediately(t *testing.T) {
	mock := &mockS3API{
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			t.Error("PutObject should not be called when file doesn't exist")
			return &s3.PutObjectOutput{}, nil
		},
	}

	client := NewClientWithAPI(mock, "test-bucket")
	client.sleepFn = func(d time.Duration) {
		t.Error("sleepFn should not be called when file doesn't exist")
	}

	err := client.Upload(context.Background(), "key", "/nonexistent/path/file.log")
	if err == nil {
		t.Fatal("Upload should have returned an error for nonexistent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected file not found error, got: %v", err)
	}
}
