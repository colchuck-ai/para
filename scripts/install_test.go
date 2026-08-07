package scripts_test

import (
	"crypto/sha256"
	"encoding/hex"
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

// The prebuilt-asset fast path (implementation plan, Phase 15, task 1). What
// it buys is the Go toolchain requirement disappearing for most users, so the
// property worth asserting is exactly that: these tests run with no `go` on
// PATH at all, and the ones that must still succeed are the ones proving the
// download path never reaches for it.
//
// The release is faked on the local filesystem and reached through --base-url,
// which is the only reason that flag exists. It is a real exercise of the URL
// layout — `releases/latest/download/<asset>` and
// `releases/download/<tag>/<asset>` — because the script builds those paths
// from the base rather than being handed them.

// fakeRelease writes a release into dir the way goreleaser lays one out: one
// archive per platform, named without the version so that
// `releases/latest/download/<asset>` resolves without knowing what the latest
// version is, plus the checksums.txt beside it. The binary inside is a script
// that reports the version it was given.
func fakeRelease(t *testing.T, dir, version string, corruptChecksum bool) {
	t.Helper()

	assets := filepath.Join(dir, "releases", "latest", "download")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}

	stage := t.TempDir()
	bin := filepath.Join(stage, "para")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'para " + version + "'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	name := assetName(t)
	archive := filepath.Join(assets, name)
	if out, err := exec.Command("tar", "-czf", archive, "-C", stage, "para").CombinedOutput(); err != nil {
		t.Fatalf("packing %s: %v\n%s", archive, err, out)
	}

	sum := "0000000000000000000000000000000000000000000000000000000000000000"
	if !corruptChecksum {
		sum = sha256Of(t, archive)
	}
	line := sum + "  " + name + "\n"
	if err := os.WriteFile(filepath.Join(assets, "checksums.txt"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assetName is the archive name the script will ask for on this machine. It is
// spelled here independently of the script, so a change to either side that
// does not change both is a test failure rather than a silent 404.
func assetName(t *testing.T) string {
	t.Helper()
	arch := runtime.GOARCH
	switch runtime.GOOS {
	case "linux", "darwin":
	default:
		t.Skipf("install.sh's prebuilt path covers linux and darwin; this is %s", runtime.GOOS)
	}
	return "para_" + runtime.GOOS + "_" + arch + ".tar.gz"
}

func sha256Of(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(sha256Sum(data))
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func TestInstallPrefersThePrebuiltAssetAndNeedsNoGo(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	release := t.TempDir()
	fakeRelease(t, release, "9.9.9-prebuilt", false)

	stdout, stderr, code := runInstaller(t, script,
		[]string{"PATH=" + pathWithoutGo(t)},
		"--dir", dir, "--base-url", "file://"+release)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	out, err := exec.Command(filepath.Join(dir, "para"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("running installed para --version: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "para 9.9.9-prebuilt" {
		t.Errorf("installed binary reports %q, want the prebuilt one", got)
	}
}

func TestInstallFallsBackToSourceWhenNoAssetIsThere(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	empty := t.TempDir() // a release directory with nothing in it: every URL 404s

	// The fallback is `go install <module>@<ref>`, which would reach the
	// network — so the toolchain is faked. What is under test is the branch,
	// not the compiler.
	stdout, stderr, code := runInstaller(t, script,
		[]string{"PATH=" + fakeGoInstallOnPath(t)},
		"--dir", dir, "--base-url", "file://"+empty)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	out, err := exec.Command(filepath.Join(dir, "para"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("running installed para --version: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "para from-source" {
		t.Errorf("installed binary reports %q, want the source build", got)
	}
	if !strings.Contains(strings.ToLower(stdout+stderr), "source") {
		t.Errorf("expected the fallback to say it is building from source, got:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

// fakeGoInstallOnPath is a `go` that reports a recent version and, for `go
// install`, writes a stub binary into $GOBIN — which is how install.sh invokes
// it. It stands in for the compiler so that the fallback branch can be tested
// without a network fetch.
func fakeGoInstallOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1" in
version) echo 'go version go1.25 darwin/amd64'; exit 0 ;;
install) printf '#!/bin/sh\necho para from-source\n' > "$GOBIN/para"; chmod 0755 "$GOBIN/para"; exit 0 ;;
esac
exit 1
`
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + pathWithoutGo(t)
}

func TestInstallRefusesAnAssetWhoseChecksumDoesNotMatch(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	release := t.TempDir()
	fakeRelease(t, release, "9.9.9-tampered", true)

	// No Go on PATH, so there is no source fallback to rescue this: a
	// checksum mismatch must fail loudly rather than quietly build instead.
	stdout, stderr, code := runInstaller(t, script,
		[]string{"PATH=" + pathWithoutGo(t)},
		"--dir", dir, "--base-url", "file://"+release)

	if code == 0 {
		t.Fatalf("expected a non-zero exit for a checksum mismatch\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !strings.Contains(strings.ToLower(stdout+stderr), "checksum") {
		t.Errorf("expected the failure to name the checksum, got:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "para")); err == nil {
		t.Error("a binary that failed its checksum must not be installed")
	}
}

func TestInstallFromSourceSkipsTheDownloadEntirely(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	release := t.TempDir()
	fakeRelease(t, release, "9.9.9-prebuilt", false)

	stdout, stderr, code := runInstaller(t, script, nil,
		"--dir", dir, "--from-source", "--repo-dir", repoRoot(t), "--base-url", "file://"+release)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	out, err := exec.Command(filepath.Join(dir, "para"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("running installed para --version: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "prebuilt") {
		t.Errorf("--from-source installed the downloaded asset: %q", out)
	}
}

func TestInstallDryRunReportsThePrebuiltAssetItWouldFetch(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	release := t.TempDir()
	fakeRelease(t, release, "9.9.9-prebuilt", false)

	stdout, stderr, code := runInstaller(t, script, []string{"PATH=" + pathWithoutGo(t)},
		"--dry-run", "--dir", dir, "--base-url", "file://"+release)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, assetName(t)) {
		t.Errorf("dry run should name the asset it would download, got: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "para")); err == nil {
		t.Error("--dry-run should not create the para binary")
	}
}

// TestReleaseNamesTheArchivesTheInstallerAsksFor is the coupling between the
// two halves of the release: goreleaser writes the archives, install.sh
// constructs their URLs, and neither reads the other.
//
// The installer's side is checked by *running* it: `--dry-run` prints the URL
// it would fetch, so the name comes from the script's own logic rather than
// from a copy of it here. goreleaser's side has to be read as text — the file
// is a template and expanding it means running goreleaser, which CI does and a
// unit test should not.
func TestReleaseNamesTheArchivesTheInstallerAsksFor(t *testing.T) {
	stdout, stderr, code := runInstaller(t, installScript(t), nil,
		"--dry-run", "--dir", t.TempDir(), "--base-url", "file:///nowhere")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	want := "file:///nowhere/releases/latest/download/" + assetName(t)
	if !strings.Contains(stdout, want) {
		t.Errorf("install.sh would fetch something other than %s:\n%s", want, stdout)
	}

	config, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	// The archive name carries no version, which is what lets the URL above be
	// built without asking an API what the latest version is.
	const nameTemplate = `name_template: "para_{{ .Os }}_{{ .Arch }}"`
	if !strings.Contains(string(config), nameTemplate) {
		t.Errorf(".goreleaser.yaml does not carry %s, which is the name install.sh builds", nameTemplate)
	}
	if !strings.Contains(string(config), "name_template: checksums.txt") {
		t.Error(".goreleaser.yaml does not publish checksums.txt, which install.sh requires before it will install anything")
	}
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
