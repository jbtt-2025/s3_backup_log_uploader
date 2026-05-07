package pathbuilder

import "testing"

func TestBuild_BasicPath(t *testing.T) {
	got := Build("logs", "web-server-01", "nginx", "", "access.log.tar.gz")
	want := "logs/web-server-01/nginx/access.log.tar.gz"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuild_WithRelPath(t *testing.T) {
	got := Build("logs", "web-server-01", "nginx", "2024/01", "error.log.tar.gz")
	want := "logs/web-server-01/nginx/2024/01/error.log.tar.gz"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuild_NoConsecutiveSlashes(t *testing.T) {
	// Even if components have leading/trailing slashes, result should be clean
	got := Build("logs/", "host", "dir", "", "file.tar.gz")
	if contains(got, "//") {
		t.Errorf("Build() = %q, contains consecutive slashes", got)
	}
}

func TestBuild_NoTrailingSlash(t *testing.T) {
	got := Build("logs", "host", "dir", "sub/", "file.tar.gz")
	if got[len(got)-1] == '/' {
		t.Errorf("Build() = %q, has trailing slash", got)
	}
}

func TestBuild_BackslashesNormalized(t *testing.T) {
	// Windows-style backslashes in relPath should be converted to forward slashes
	got := Build("logs", "host", "dir", "sub\\path", "file.tar.gz")
	want := "logs/host/dir/sub/path/file.tar.gz"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuild_EmptyRelPathOmitted(t *testing.T) {
	got := Build("base", "myhost", "applog", "", "app.log.tar.gz")
	want := "base/myhost/applog/app.log.tar.gz"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuild_DeepRelPath(t *testing.T) {
	got := Build("backup", "srv1", "syslog", "2024/06/15", "messages.tar.gz")
	want := "backup/srv1/syslog/2024/06/15/messages.tar.gz"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuild_AlreadyCompressedFile(t *testing.T) {
	got := Build("logs", "web-server-01", "nginx", "", "access.log.gz")
	want := "logs/web-server-01/nginx/access.log.gz"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
