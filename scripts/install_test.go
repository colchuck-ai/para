package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the checkout this test file lives in, so tests can build
// para from local source rather than reaching out to the network.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile is .../scripts/install_test.go; the repo root is one level up.
	return filepath.Dir(filepath.Dir(thisFile))
}

func installScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "scripts", "install.sh")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("install.sh not found at %s: %v", path, err)
	}
	return path
}

// fakeGoOnPath prepends a directory containing a fake `go` script (reporting
// version) to PATH, returning the resulting PATH value.
func fakeGoOnPath(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo 'go version " + version + " darwin/amd64'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// pathWithoutGo returns a PATH value with every directory containing a `go`
// executable removed, so the installer sees no Go toolchain at all.
func pathWithoutGo(t *testing.T) string {
	t.Helper()
	var kept []string
	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "go")); err == nil {
			continue
		}
		kept = append(kept, dir)
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

func runInstaller(t *testing.T, path string, extraEnv []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{path}, args...)...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running install.sh: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

func TestInstallDryRunDoesNotInstall(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()

	stdout, stderr, code := runInstaller(t, script, nil,
		"--dry-run", "--dir", dir, "--repo-dir", repoRoot(t))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "para")); err == nil {
		t.Error("--dry-run should not create the para binary")
	}
	if !strings.Contains(stdout, dir) {
		t.Errorf("dry-run output should mention the target dir %q, got: %s", dir, stdout)
	}
}

func TestInstallBuildsFromLocalCheckout(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()

	stdout, stderr, code := runInstaller(t, script, nil,
		"--dir", dir, "--repo-dir", repoRoot(t))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	bin := filepath.Join(dir, "para")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("expected %s to exist: %v", bin, err)
	}

	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("running installed para --version: %v\n%s", err, out)
	}
	if !strings.HasPrefix(string(out), "para ") {
		t.Errorf("installed para --version output = %q, want prefix %q", out, "para ")
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()

	for i := 0; i < 2; i++ {
		_, stderr, code := runInstaller(t, script, nil, "--dir", dir, "--repo-dir", repoRoot(t))
		if code != 0 {
			t.Fatalf("run %d: exit code = %d, want 0\nstderr: %s", i, code, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "para")); err != nil {
		t.Fatalf("expected para binary to exist after repeated installs: %v", err)
	}
}

func TestInstallWarnsWhenDirOffPath(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir() // a fresh temp dir is guaranteed not to already be on PATH

	stdout, stderr, code := runInstaller(t, script, nil, "--dir", dir, "--repo-dir", repoRoot(t))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(strings.ToUpper(combined), "PATH") {
		t.Errorf("expected a warning mentioning PATH, got:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

func TestInstallFailsClearlyWhenGoMissing(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()

	stdout, stderr, code := runInstaller(t, script, []string{"PATH=" + pathWithoutGo(t)},
		"--dir", dir, "--repo-dir", repoRoot(t))

	if code == 0 {
		t.Fatalf("expected non-zero exit when go is missing\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	combined := strings.ToLower(stdout + stderr)
	if !strings.Contains(combined, "go") {
		t.Errorf("expected error to mention the missing Go toolchain, got:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

func TestInstallFailsClearlyWhenGoTooOld(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()

	stdout, stderr, code := runInstaller(t, script, []string{"PATH=" + fakeGoOnPath(t, "go1.10")},
		"--dir", dir, "--repo-dir", repoRoot(t))

	if code == 0 {
		t.Fatalf("expected non-zero exit for a too-old Go toolchain\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	combined := strings.ToLower(stdout + stderr)
	if !strings.Contains(combined, "1.24") && !strings.Contains(combined, "go version") {
		t.Errorf("expected error to explain the version requirement, got:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

func TestUninstallRemovesBinaryAndIsIdempotent(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()

	if _, stderr, code := runInstaller(t, script, nil, "--dir", dir, "--repo-dir", repoRoot(t)); code != 0 {
		t.Fatalf("initial install failed: %s", stderr)
	}

	for i := 0; i < 2; i++ {
		_, stderr, code := runInstaller(t, script, nil, "--uninstall", "--dir", dir)
		if code != 0 {
			t.Fatalf("uninstall run %d: exit code = %d, want 0\nstderr: %s", i, code, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "para")); err == nil {
		t.Error("--uninstall should have removed the para binary")
	}
}
