package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

// TestMoveFileBasic moves a file into a new (not yet existing) target
// directory. With UseNil the extension check is skipped and oknorename is
// true, so the original filename is kept regardless of newname.
func TestMoveFileBasic(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	writeTestFile(t, src, "content-basic")

	target := filepath.Join(dir, "sub", "target")

	newpath, err := MoveFile(src, nil, target, "newname", MoveFileOptions{UseNil: true})
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	if want := filepath.Join(target, "source.mkv"); newpath != want {
		t.Fatalf("newpath = %q; want %q", newpath, want)
	}

	if got := readTestFile(t, newpath); got != "content-basic" {
		t.Fatalf("content = %q; want %q", got, "content-basic")
	}

	if CheckFileExist(src) {
		t.Fatal("source file still exists after move")
	}

	if CheckFileExist(newpath + tmpMoveSuffix) {
		t.Fatal("staging file left behind after move")
	}
}

// TestMoveFileReplacesExisting verifies an existing destination is replaced
// by the new content.
func TestMoveFileReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	writeTestFile(t, src, "new-content")

	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o777); err != nil {
		t.Fatal(err)
	}

	existing := filepath.Join(target, "source.mkv")
	writeTestFile(t, existing, "old-content")

	newpath, err := MoveFile(src, nil, target, "", MoveFileOptions{UseNil: true})
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	if got := readTestFile(t, newpath); got != "new-content" {
		t.Fatalf("content = %q; want %q", got, "new-content")
	}
}

// TestMoveFileOntoItself must be a no-op and must not delete the file.
func TestMoveFileOntoItself(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	writeTestFile(t, src, "keep-me")

	newpath, err := MoveFile(src, nil, dir, "", MoveFileOptions{UseNil: true})
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	if !CheckFileExist(src) {
		t.Fatal("file was deleted when moved onto itself")
	}

	if got := readTestFile(t, newpath); got != "keep-me" {
		t.Fatalf("content = %q; want %q", got, "keep-me")
	}
}

// TestMoveFileStaleTemp verifies a leftover staging file from an interrupted
// move does not block a retry.
func TestMoveFileStaleTemp(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	writeTestFile(t, src, "fresh")

	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(target, "source.mkv")+tmpMoveSuffix, "stale partial")

	newpath, err := MoveFile(src, nil, target, "", MoveFileOptions{UseNil: true})
	if err != nil {
		t.Fatalf("MoveFile failed with stale temp present: %v", err)
	}

	if got := readTestFile(t, newpath); got != "fresh" {
		t.Fatalf("content = %q; want %q", got, "fresh")
	}
}

// TestCopyFileVerified checks content, size verification, and mtime preservation.
func TestCopyFileVerified(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	writeTestFile(t, src, "copy-payload")

	past := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(src, past, past); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "dst.bin")
	if err := copyFileVerified(src, dst); err != nil {
		t.Fatalf("copyFileVerified failed: %v", err)
	}

	if got := readTestFile(t, dst); got != "copy-payload" {
		t.Fatalf("content = %q; want %q", got, "copy-payload")
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}

	if !info.ModTime().Truncate(time.Second).Equal(past) {
		t.Fatalf("mtime = %v; want %v (source mtime preserved)", info.ModTime(), past)
	}
}

// TestCopyFileVerifiedNonRegular rejects directories as source.
func TestCopyFileVerifiedNonRegular(t *testing.T) {
	dir := t.TempDir()

	if err := copyFileVerified(dir, filepath.Join(dir, "out.bin")); err == nil {
		t.Fatal("expected error copying a directory")
	}
}

// TestMoveFolder moves a folder tree via the rename fast path.
func TestMoveFolder(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")

	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(src, "a.txt"), "a")
	writeTestFile(t, filepath.Join(src, "sub", "b.txt"), "b")

	dst := filepath.Join(dir, "moved")
	if err := MoveFolder(src, dst); err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}

	if got := readTestFile(t, filepath.Join(dst, "sub", "b.txt")); got != "b" {
		t.Fatalf("content = %q; want %q", got, "b")
	}

	if CheckFileExist(src) {
		t.Fatal("source folder still exists")
	}
}

// TestFreeDiskSpace sanity-checks the platform implementation.
func TestFreeDiskSpace(t *testing.T) {
	free := freeDiskSpace(t.TempDir())
	if free <= 0 {
		t.Fatalf("freeDiskSpace = %d; want > 0 for the temp directory", free)
	}
}
