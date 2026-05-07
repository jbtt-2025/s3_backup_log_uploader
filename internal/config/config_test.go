package config

import (
	"errors"
	"strings"
	"testing"
)

// mockEnvReader implements interfaces.EnvReader for testing
type mockEnvReader struct {
	vars map[string]string
}

func (m *mockEnvReader) Getenv(key string) string {
	return m.vars[key]
}

// mockOSHost implements interfaces.OSHost for testing
type mockOSHost struct {
	hostname string
	err      error
}

func (m *mockOSHost) Hostname() (string, error) {
	return m.hostname, m.err
}

func validEnv() map[string]string {
	return map[string]string{
		"AK":       "test-ak",
		"SK":       "test-sk",
		"ENDPOINT": "https://abc123.r2.cloudflarestorage.com/my-bucket",
		"BASE_DIR": "logs",
		"LOG_DICT": "/var/log/nginx;/var/log/app",
	}
}

func TestLoad_Success(t *testing.T) {
	env := &mockEnvReader{vars: validEnv()}
	host := &mockOSHost{hostname: "web-server-01"}

	cfg, err := Load(env, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AK != "test-ak" {
		t.Errorf("AK = %q, want %q", cfg.AK, "test-ak")
	}
	if cfg.SK != "test-sk" {
		t.Errorf("SK = %q, want %q", cfg.SK, "test-sk")
	}
	if cfg.Endpoint != "https://abc123.r2.cloudflarestorage.com" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "https://abc123.r2.cloudflarestorage.com")
	}
	if cfg.Bucket != "my-bucket" {
		t.Errorf("Bucket = %q, want %q", cfg.Bucket, "my-bucket")
	}
	if cfg.BaseDir != "logs" {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, "logs")
	}
	if len(cfg.LogDirs) != 2 || cfg.LogDirs[0] != "/var/log/nginx" || cfg.LogDirs[1] != "/var/log/app" {
		t.Errorf("LogDirs = %v, want [/var/log/nginx /var/log/app]", cfg.LogDirs)
	}
	if cfg.Hostname != "web-server-01" {
		t.Errorf("Hostname = %q, want %q", cfg.Hostname, "web-server-01")
	}
	if cfg.IgnoreDays != 0 {
		t.Errorf("IgnoreDays = %d, want 0", cfg.IgnoreDays)
	}
}

func TestLoad_IgnoreDaysValid(t *testing.T) {
	vars := validEnv()
	vars["IGNORE_DAYS"] = "7"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	cfg, err := Load(env, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IgnoreDays != 7 {
		t.Errorf("IgnoreDays = %d, want 7", cfg.IgnoreDays)
	}
}

func TestLoad_IgnoreDaysZero(t *testing.T) {
	vars := validEnv()
	vars["IGNORE_DAYS"] = "0"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	cfg, err := Load(env, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IgnoreDays != 0 {
		t.Errorf("IgnoreDays = %d, want 0", cfg.IgnoreDays)
	}
}

func TestLoad_IgnoreDaysNegative(t *testing.T) {
	vars := validEnv()
	vars["IGNORE_DAYS"] = "-1"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for negative IGNORE_DAYS")
	}
	if !strings.Contains(err.Error(), "IGNORE_DAYS") {
		t.Errorf("error should mention IGNORE_DAYS: %v", err)
	}
}

func TestLoad_IgnoreDaysInvalid(t *testing.T) {
	vars := validEnv()
	vars["IGNORE_DAYS"] = "abc"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for invalid IGNORE_DAYS")
	}
	if !strings.Contains(err.Error(), "IGNORE_DAYS") {
		t.Errorf("error should mention IGNORE_DAYS: %v", err)
	}
}

func TestLoad_MissingAllRequired(t *testing.T) {
	env := &mockEnvReader{vars: map[string]string{}}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for missing variables")
	}

	errMsg := err.Error()
	for _, v := range []string{"AK", "SK", "ENDPOINT", "BASE_DIR", "LOG_DICT"} {
		if !strings.Contains(errMsg, v) {
			t.Errorf("error should mention %q: %v", v, err)
		}
	}
}

func TestLoad_MissingSomeRequired(t *testing.T) {
	vars := validEnv()
	delete(vars, "AK")
	delete(vars, "SK")
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for missing variables")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "AK") {
		t.Errorf("error should mention AK: %v", err)
	}
	if !strings.Contains(errMsg, "SK") {
		t.Errorf("error should mention SK: %v", err)
	}
	// Should NOT mention variables that are present
	if strings.Contains(errMsg, "ENDPOINT") {
		t.Errorf("error should not mention ENDPOINT: %v", err)
	}
}

func TestLoad_EndpointNoBucket(t *testing.T) {
	vars := validEnv()
	vars["ENDPOINT"] = "https://abc123.r2.cloudflarestorage.com"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for ENDPOINT without bucket")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Errorf("error should mention bucket: %v", err)
	}
}

func TestLoad_EndpointOnlySlash(t *testing.T) {
	vars := validEnv()
	vars["ENDPOINT"] = "https://abc123.r2.cloudflarestorage.com/"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for ENDPOINT with only slash")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Errorf("error should mention bucket: %v", err)
	}
}

func TestLoad_EndpointWithExtraPath(t *testing.T) {
	vars := validEnv()
	vars["ENDPOINT"] = "https://abc123.r2.cloudflarestorage.com/my-bucket/extra/path"
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	cfg, err := Load(env, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only take the first path segment as bucket
	if cfg.Bucket != "my-bucket" {
		t.Errorf("Bucket = %q, want %q", cfg.Bucket, "my-bucket")
	}
}

func TestLoad_LogDictWithWhitespace(t *testing.T) {
	vars := validEnv()
	vars["LOG_DICT"] = "  /var/log/nginx ; /var/log/app  ;  ;  /tmp/logs  "
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	cfg, err := Load(env, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"/var/log/nginx", "/var/log/app", "/tmp/logs"}
	if len(cfg.LogDirs) != len(expected) {
		t.Fatalf("LogDirs length = %d, want %d", len(cfg.LogDirs), len(expected))
	}
	for i, dir := range cfg.LogDirs {
		if dir != expected[i] {
			t.Errorf("LogDirs[%d] = %q, want %q", i, dir, expected[i])
		}
	}
}

func TestLoad_LogDictAllEmpty(t *testing.T) {
	vars := validEnv()
	vars["LOG_DICT"] = "  ;  ;  "
	env := &mockEnvReader{vars: vars}
	host := &mockOSHost{hostname: "server-01"}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for empty LOG_DICT after parsing")
	}
	if !strings.Contains(err.Error(), "LOG_DICT") {
		t.Errorf("error should mention LOG_DICT: %v", err)
	}
}

func TestLoad_HostnameError(t *testing.T) {
	env := &mockEnvReader{vars: validEnv()}
	host := &mockOSHost{hostname: "", err: errors.New("hostname lookup failed")}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for hostname failure")
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error should mention hostname: %v", err)
	}
}

func TestLoad_HostnameEmpty(t *testing.T) {
	env := &mockEnvReader{vars: validEnv()}
	host := &mockOSHost{hostname: "", err: nil}

	_, err := Load(env, host)
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error should mention hostname: %v", err)
	}
}

func TestParseLogDict(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple", "/a;/b", []string{"/a", "/b"}},
		{"with spaces", " /a ; /b ", []string{"/a", "/b"}},
		{"empty entries", "/a;;/b", []string{"/a", "/b"}},
		{"all empty", ";;;", nil},
		{"single", "/var/log", []string{"/var/log"}},
		{"trailing semicolons", "/a;/b;", []string{"/a", "/b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLogDict(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("ParseLogDict(%q) = %v (len %d), want %v (len %d)",
					tt.input, result, len(result), tt.expected, len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("ParseLogDict(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}
