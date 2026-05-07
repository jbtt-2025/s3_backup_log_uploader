package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"log-uploader/internal/interfaces"
)

// Config 保存所有运行时配置
type Config struct {
	AK         string   // R2 Access Key ID
	SK         string   // R2 Secret Access Key
	Endpoint   string   // R2 Endpoint URL (scheme + host)
	Bucket     string   // R2 Bucket 名称
	BaseDir    string   // 上传路径前缀
	LogDirs    []string // 日志目录列表
	Hostname   string   // 机器主机名
	IgnoreDays int      // 忽略超过指定天数未修改的文件，0 表示不忽略
	Workers    int      // 并发上传 worker 数量，默认 1
}

// Load 从环境变量加载配置，验证必填项，获取主机名。
// ENDPOINT 环境变量格式为 https://{account_id}.r2.cloudflarestorage.com/{bucket}，
// 程序自动拆分为 Endpoint（去除路径）和 Bucket（路径第一段）。
// 返回错误时包含所有缺失变量的名称列表。
func Load(env interfaces.EnvReader, host interfaces.OSHost) (*Config, error) {
	// Read all required environment variables
	ak := env.Getenv("AK")
	sk := env.Getenv("SK")
	endpoint := env.Getenv("ENDPOINT")
	baseDir := env.Getenv("BASE_DIR")
	logDict := env.Getenv("LOG_DICT")
	ignoreDaysStr := env.Getenv("IGNORE_DAYS")

	// Validate required variables - collect all missing ones
	var missing []string
	if ak == "" {
		missing = append(missing, "AK")
	}
	if sk == "" {
		missing = append(missing, "SK")
	}
	if endpoint == "" {
		missing = append(missing, "ENDPOINT")
	}
	if baseDir == "" {
		missing = append(missing, "BASE_DIR")
	}
	if logDict == "" {
		missing = append(missing, "LOG_DICT")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// Parse ENDPOINT URL into endpoint address and bucket name
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid ENDPOINT URL: %v", err)
	}

	// Extract bucket from path (first segment)
	path := strings.TrimPrefix(parsedURL.Path, "/")
	if path == "" {
		return nil, fmt.Errorf("ENDPOINT URL has no bucket name in path: %s", endpoint)
	}
	// Take the first path segment as bucket name
	bucket := strings.SplitN(path, "/", 2)[0]

	// Construct endpoint URL (scheme + host only)
	endpointURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Parse LOG_DICT: split by semicolons, trim whitespace, discard empty entries
	logDirs := ParseLogDict(logDict)
	if len(logDirs) == 0 {
		return nil, fmt.Errorf("LOG_DICT contains no valid directory paths after parsing")
	}

	// Parse IGNORE_DAYS (optional, default 0)
	ignoreDays := 0
	if ignoreDaysStr != "" {
		ignoreDays, err = strconv.Atoi(ignoreDaysStr)
		if err != nil || ignoreDays < 0 {
			return nil, fmt.Errorf("IGNORE_DAYS must be a non-negative integer, got: %q", ignoreDaysStr)
		}
	}

	// Parse WORKERS (optional, default 1)
	workersStr := env.Getenv("WORKERS")
	workers := 1
	if workersStr != "" {
		workers, err = strconv.Atoi(workersStr)
		if err != nil || workers < 1 {
			return nil, fmt.Errorf("WORKERS must be a positive integer, got: %q", workersStr)
		}
	}

	// Get hostname
	hostname, err := host.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}
	if hostname == "" {
		return nil, fmt.Errorf("hostname is empty")
	}

	return &Config{
		AK:         ak,
		SK:         sk,
		Endpoint:   endpointURL,
		Bucket:     bucket,
		BaseDir:    baseDir,
		LogDirs:    logDirs,
		Hostname:   hostname,
		IgnoreDays: ignoreDays,
		Workers:    workers,
	}, nil
}

// ParseLogDict splits a semicolon-separated string into a list of directory paths,
// trimming whitespace and discarding empty entries.
func ParseLogDict(logDict string) []string {
	parts := strings.Split(logDict, ";")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
