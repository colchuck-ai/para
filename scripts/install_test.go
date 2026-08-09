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

// TestMain skips this whole package on Windows.
//
// install.sh is a POSIX shell script and these tests run it as one: they exec
// `sh`, plant `#!/bin/sh` stubs on PATH, filter PATH by looking for a `go` with
// no extension, and stat the installed binary as `para`. Every one of those is
// a Unix assumption, and on Windows each fails as a *test* bug rather than as a
// finding about the installer — `pathWithoutGo` does not remove a directory
// holding `go.exe`, so "fails clearly when Go is missing" would fail with Go
// very much present.
//
// It is a runtime skip rather than a `//go:build !windows` constraint because
// every file in this package is a test file: excluding them all leaves a
// package with no Go files, which `go test ./...` reports as an error rather
// than passing over.
//
// Windows users get the prebuilt `.zip` from the release page and never run
// install.sh, so there is nothing here to port — only the asset naming, which
// assetName already covers on the platforms that have a `.tar.gz`.
func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

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

// pathWithoutGo returns a PATH value with the Go toolchain removed, so the
// installer sees no `go` at all.
//
// Dropping every directory that holds a `go` takes everything else in that
// directory with it, and on the GitHub ubuntu runner that meant `uname` and
// `tr`: install.sh died at its platform check with `uname: not found`, and five
// tests failed for a reason with nothing to do with the Go toolchain being
// absent. Removing the *directory* was never the goal; removing one executable
// from it was.
//
// So a directory holding only the toolchain — `…/go/bin`, which is `go` and
// `gofmt` and nothing else — is dropped outright, and a directory that holds
// `go` beside anything else is replaced by a shim: a temporary directory of
// symlinks to every entry except `go`. The shell keeps its utilities, the
// installer finds no toolchain, and neither outcome depends on where the
// machine happens to install Go.
func pathWithoutGo(t *testing.T) string {
	t.Helper()
	var kept []string
	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "go")); err != nil {
			kept = append(kept, dir)
			continue
		}
		if shim, ok := shimWithoutGo(t, dir); ok {
			kept = append(kept, shim)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// TestPathWithoutGoKeepsTheShellsUtilities pins both halves of the helper's
// contract, because getting one of them wrong is what turned a green suite red
// on CI and nowhere else.
//
// The two directory shapes are exercised deliberately: a toolchain directory,
// which is what a developer machine usually has and is why this was invisible
// locally, and a shared directory holding `go` beside a utility, which is what
// the runner had. The old helper dropped the second one whole and install.sh
// died at `uname: not found`.
func TestPathWithoutGoKeepsTheShellsUtilities(t *testing.T) {
	stub := func(dir, name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	shared := t.TempDir()
	stub(shared, "go")
	stub(shared, "uname")
	stub(shared, "tr")

	toolchain := t.TempDir()
	stub(toolchain, "go")
	stub(toolchain, "gofmt")

	t.Setenv("PATH", shared+string(os.PathListSeparator)+toolchain)

	var sawUname, sawTr bool
	for _, dir := range strings.Split(pathWithoutGo(t), string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		// Stat, not Lstat: a shim entry is a symlink and what matters is what
		// the shell would find at the end of it.
		if _, err := os.Stat(filepath.Join(dir, "go")); err == nil {
			t.Errorf("%s still offers go", dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "uname")); err == nil {
			sawUname = true
		}
		if _, err := os.Stat(filepath.Join(dir, "tr")); err == nil {
			sawTr = true
		}
	}
	if !sawUname || !sawTr {
		t.Errorf("the shell's utilities did not survive: uname=%v tr=%v", sawUname, sawTr)
	}
}

// shimWithoutGo mirrors dir into a temporary directory, leaving `go` out. It
// reports false when there is nothing worth keeping, which is the plain
// toolchain-directory case.
func shimWithoutGo(t *testing.T, dir string) (string, bool) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Unreadable is indistinguishable from empty for this purpose, and a
		// PATH entry that cannot be read contributes no `go` either.
		return "", false
	}
	shim := t.TempDir()
	kept := 0
	for _, e := range entries {
		name := e.Name()
		// `gofmt` goes too: it is the toolchain, and leaving it behind in a
		// directory whose whole point is "no Go here" would be confusing to
		// anyone debugging a failure.
		if name == "go" || name == "gofmt" {
			continue
		}
		if err := os.Symlink(filepath.Join(dir, name), filepath.Join(shim, name)); err != nil {
			continue
		}
		kept++
	}
	return shim, kept > 0
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
	fakeReleaseAt(t, dir, filepath.Join("releases", "latest", "download"), version, corruptChecksum)
}

// fakeReleaseTagged writes the same release under the layout an explicit
// `--ref v1.2.3` asks for: `releases/download/<tag>/<asset>`. It is a separate
// helper because it is a separate URL shape, and the header's claim to exercise
// both was untrue until something actually wrote this one.
func fakeReleaseTagged(t *testing.T, dir, tag, version string) {
	t.Helper()
	fakeReleaseAt(t, dir, filepath.Join("releases", "download", tag), version, false)
}

func fakeReleaseAt(t *testing.T, dir, sub, version string, corruptChecksum bool) {
	t.Helper()

	assets := filepath.Join(dir, sub)
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

// TestInstallRefPinsTheTaggedRelease covers the second of the two URL layouts
// the header claims: an explicit `--ref v9.9.9` asks for
// `releases/download/v9.9.9/<asset>`, not for the latest release.
//
// The two are asserted apart rather than together: the tagged release is the
// only one written, so an installer that still built a `latest` URL would 404
// and fall back to source instead of installing 9.9.9-tagged.
func TestInstallRefPinsTheTaggedRelease(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	release := t.TempDir()
	fakeReleaseTagged(t, release, "v9.9.9", "9.9.9-tagged")

	stdout, stderr, code := runInstaller(t, script,
		[]string{"PATH=" + pathWithoutGo(t)},
		"--dir", dir, "--ref", "v9.9.9", "--base-url", "file://"+release)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	out, err := exec.Command(filepath.Join(dir, "para"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("running installed para --version: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "para 9.9.9-tagged" {
		t.Errorf("installed binary reports %q, want the tagged release", got)
	}
}

// TestInstallRefMainBuildsMainRatherThanTheLatestRelease pins the distinction
// the flag's own help draws: `--ref` names "a release tag to install, or a git
// ref to build from when no release matches", and `main` is the second kind.
//
// The default is also the string "main", and folding the two made an explicit
// `--ref main` silently install the newest *release* — the one thing the flag
// says it does not do. A latest-layout release is planted here precisely so
// that taking it would be visible.
//
// It asserts through --dry-run rather than through an installed binary, and
// that is not a shortcut: --repo-dir is what the other source-path tests use to
// stay off the network, and install.sh skips the whole prebuilt branch when it
// is set — so a test that passed --repo-dir could not reach the decision it is
// about. --dry-run reports which branch was chosen and reaches the network no
// more than the release directory does.
func TestInstallRefMainBuildsMainRatherThanTheLatestRelease(t *testing.T) {
	script := installScript(t)
	dir := t.TempDir()
	release := t.TempDir()
	fakeRelease(t, release, "9.9.9-prebuilt", false)

	stdout, stderr, code := runInstaller(t, script, nil,
		"--dir", dir, "--ref", "main", "--dry-run", "--base-url", "file://"+release)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// The observable decision is which URL was built. --dry-run reports the
	// asset it would reach for without checking that it is there, so the
	// latest-vs-tagged choice is visible in the path and the 404-then-fall-back
	// that follows it is not.
	combined := stdout + stderr
	if strings.Contains(combined, "releases/latest/download") {
		t.Errorf("--ref main reached for the latest release:\n%s", combined)
	}
	if !strings.Contains(combined, "releases/download/main") {
		t.Errorf("--ref main did not treat main as a ref of its own:\n%s", combined)
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
	// The whole message, not the substring "go" — which every $TMPDIR path
	// containing the letters would have satisfied, including a message about
	// something else entirely. What the user needs is the diagnosis and where
	// to go, so both are asserted.
	combined := stdout + stderr
	if !strings.Contains(combined, "Go toolchain not found on PATH") {
		t.Errorf("expected the missing-toolchain diagnosis, got:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !strings.Contains(combined, "https://go.dev/dl/") {
		t.Errorf("expected the message to say where to get Go, got:\nstdout: %s\nstderr: %s", stdout, stderr)
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
