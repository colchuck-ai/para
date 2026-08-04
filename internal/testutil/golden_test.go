package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenPassesOnMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.golden")
	if err := os.WriteFile(path, []byte("want\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	spy := &fakeT{}
	Golden(spy, path, []byte("want\n"))
	if spy.failed {
		t.Error("Golden() failed when content matches the golden file")
	}
}

func TestGoldenFailsOnMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.golden")
	if err := os.WriteFile(path, []byte("want\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	spy := &fakeT{}
	Golden(spy, path, []byte("got\n"))
	if !spy.failed {
		t.Error("Golden() should fail when content does not match the golden file")
	}
}

func TestGoldenFailsWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.golden")

	spy := &fakeT{}
	Golden(spy, path, []byte("anything\n"))
	if !spy.failed {
		t.Error("Golden() should fail when the golden file does not exist")
	}
}

func TestGoldenUpdateWritesAndThenMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "example.golden")

	*update = true
	writeSpy := &fakeT{}
	Golden(writeSpy, path, []byte("fresh content\n"))
	*update = false
	if writeSpy.failed {
		t.Fatal("Golden() with -update should not fail")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file Golden() should have created: %v", err)
	}
	if string(got) != "fresh content\n" {
		t.Errorf("golden file content = %q, want %q", got, "fresh content\n")
	}

	verifySpy := &fakeT{}
	Golden(verifySpy, path, []byte("fresh content\n"))
	if verifySpy.failed {
		t.Error("Golden() should pass once the updated file is read back")
	}
}
