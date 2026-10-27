package plugin

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// #3905: file:///C:/… must not keep the leading slash that Abs then joins
// onto the working drive as C:\C:\….
func TestFileURLLocalPathWindowsDrive(t *testing.T) {
	u, err := url.Parse("file:///C:/Users/me/plugins/dist/pkg.tgz")
	if err != nil {
		t.Fatal(err)
	}
	got := fileURLLocalPath(u, "windows")
	if strings.HasPrefix(got, `/C:`) || strings.HasPrefix(got, `\C:`) {
		t.Fatalf("fileURLLocalPath(%q) = %q, still has a leading separator before the drive", u, got)
	}
	if !strings.HasPrefix(got, "C:") {
		t.Fatalf("fileURLLocalPath(%q) = %q, want a path that starts with C:", u, got)
	}
	if want := filepath.FromSlash("C:/Users/me/plugins/dist/pkg.tgz"); got != want {
		// On Windows FromSlash yields backslashes; on other GOOS it keeps /.
		t.Fatalf("fileURLLocalPath(%q) = %q, want %q", u, got, want)
	}
}

func TestFileURLLocalPathPOSIX(t *testing.T) {
	u, err := url.Parse("file:///Users/me/plugin")
	if err != nil {
		t.Fatal(err)
	}
	got := fileURLLocalPath(u, "linux")
	if got != "/Users/me/plugin" {
		t.Fatalf("fileURLLocalPath(%q) = %q, want /Users/me/plugin", u, got)
	}
}

func TestFileURLLocalPathWindowsUNC(t *testing.T) {
	u, err := url.Parse("file://server/share/plugin")
	if err != nil {
		t.Fatal(err)
	}
	got := fileURLLocalPath(u, "windows")
	want := `\\server` + filepath.FromSlash("/share/plugin")
	if got != want {
		t.Fatalf("fileURLLocalPath(%q) = %q, want %q", u, got, want)
	}
}

// On this machine, a file:// of a real temp dir must still resolve through Target.
func TestTargetFileURLOfTempDir(t *testing.T) {
	dir := t.TempDir()
	spec := "file://" + filepath.ToSlash(dir)
	if runtime.GOOS == "windows" && !strings.HasPrefix(spec, "file:///") {
		// url.Parse wants file:///C:/… on Windows.
		spec = "file:///" + strings.TrimPrefix(filepath.ToSlash(dir), "/")
	}
	got := Target(spec)
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("Target(%q) = %q, want %q", spec, got, abs)
	}
}
