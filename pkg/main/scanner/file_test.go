package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if err := MoveFolder(src, dst, MoveFileOptions{}); err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}

	if got := readTestFile(t, filepath.Join(dst, "sub", "b.txt")); got != "b" {
		t.Fatalf("content = %q; want %q", got, "b")
	}

	if CheckFileExist(src) {
		t.Fatal("source folder still exists")
	}
}

// noStagingLeftovers fails when dir contains any ".tmp-move" staging file.
func noStagingLeftovers(t *testing.T, dir string) {
	t.Helper()

	err := filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(filepath.Base(path), tmpMoveSuffix) {
			t.Errorf("staging file left behind: %s", path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestMoveFileNoStagingLeftovers checks the unique staging file is gone after a move.
func TestMoveFileNoStagingLeftovers(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	writeTestFile(t, src, "x")

	target := filepath.Join(dir, "target")
	if _, err := MoveFile(src, nil, target, "", MoveFileOptions{UseNil: true}); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	noStagingLeftovers(t, target)
}

// TestUniqueTmpPath verifies staging names differ per call and never end in a
// media extension.
func TestUniqueTmpPath(t *testing.T) {
	a := uniqueTmpPath("/x/movie.mkv")
	b := uniqueTmpPath("/x/movie.mkv")

	if a == b {
		t.Fatalf("staging names collide: %q", a)
	}

	for _, p := range []string{a, b} {
		if !strings.HasPrefix(p, "/x/movie.mkv"+tmpMoveSuffix+"-") {
			t.Fatalf("unexpected staging name %q", p)
		}

		if !strings.HasPrefix(filepath.Ext(p), tmpMoveSuffix) {
			t.Fatalf("staging name %q has extension %q", p, filepath.Ext(p))
		}
	}
}

// TestCopyFileVerifiedRefusesExisting must never truncate an existing file.
func TestCopyFileVerifiedRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	writeTestFile(t, src, "new")
	writeTestFile(t, dst, "keep")

	if err := copyFileVerified(src, dst); err == nil {
		t.Fatal("expected error copying onto an existing file")
	}

	if got := readTestFile(t, dst); got != "keep" {
		t.Fatalf("existing destination changed to %q", got)
	}
}

// forceCopyFallback makes dst a non-empty directory so os.Rename(src, dst)
// fails and MoveFolder takes the copy path.
func forceCopyFallback(t *testing.T, dst string) {
	t.Helper()

	if err := os.MkdirAll(dst, 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(dst, "zz-existing.txt"), "existing")
}

// TestMoveFolderCopyFailureKeepsExisting: a failed copy must neither touch a
// pre-existing destination file nor remove the source.
func TestMoveFolderCopyFailureKeepsExisting(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read unreadable files")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	dst := filepath.Join(dir, "moved")

	if err := os.MkdirAll(src, 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(src, "a.txt"), "a")
	writeTestFile(t, filepath.Join(src, "b.txt"), "b-new")

	if err := os.Chmod(filepath.Join(src, "b.txt"), 0); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Chmod(filepath.Join(src, "b.txt"), 0o644) })

	forceCopyFallback(t, dst)
	writeTestFile(t, filepath.Join(dst, "b.txt"), "b-keep")

	if err := MoveFolder(src, dst, MoveFileOptions{}); err == nil {
		t.Fatal("expected MoveFolder to fail")
	}

	if got := readTestFile(t, filepath.Join(dst, "b.txt")); got != "b-keep" {
		t.Fatalf("pre-existing destination file changed to %q", got)
	}

	if got := readTestFile(t, filepath.Join(dst, "zz-existing.txt")); got != "existing" {
		t.Fatalf("unrelated destination file changed to %q", got)
	}

	if !CheckFileExist(filepath.Join(src, "a.txt")) || !CheckFileExist(filepath.Join(src, "b.txt")) {
		t.Fatal("source removed after failed folder move")
	}

	noStagingLeftovers(t, dst)
}

// TestMoveFolderTargetFileIsDir: the rename into place fails, the temp copy is
// cleaned up and the source is kept.
func TestMoveFolderTargetFileIsDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	dst := filepath.Join(dir, "moved")

	if err := os.MkdirAll(src, 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(src, "x.txt"), "x")

	if err := os.MkdirAll(filepath.Join(dst, "x.txt", "inner"), 0o777); err != nil {
		t.Fatal(err)
	}

	if err := MoveFolder(src, dst, MoveFileOptions{}); err == nil {
		t.Fatal("expected MoveFolder to fail")
	}

	if got := readTestFile(t, filepath.Join(src, "x.txt")); got != "x" {
		t.Fatalf("source content = %q", got)
	}

	noStagingLeftovers(t, dst)
}

// TestMoveFolderCopyReplacesAndRemovesSource covers a successful copy-path move
// merging into an existing destination with the configured modes applied.
func TestMoveFolderCopyReplacesAndRemovesSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	dst := filepath.Join(dir, "moved")

	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(src, "sub", "b.txt"), "b-new")
	forceCopyFallback(t, dst)

	err := MoveFolder(src, dst, MoveFileOptions{Chmod: "0640", ChmodFolder: "0750"})
	if err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}

	if got := readTestFile(t, filepath.Join(dst, "sub", "b.txt")); got != "b-new" {
		t.Fatalf("content = %q", got)
	}

	if CheckFileExist(src) {
		t.Fatal("source folder still exists")
	}

	if fi, err := os.Stat(filepath.Join(dst, "sub")); err != nil || fi.Mode().Perm() != 0o750 {
		t.Fatalf("created dir mode = %v (%v); want 0750", fi.Mode().Perm(), err)
	}

	if fi, err := os.Stat(filepath.Join(dst, "sub", "b.txt")); err != nil || fi.Mode().Perm() != 0o640 {
		t.Fatalf("copied file mode = %v (%v); want 0640", fi.Mode().Perm(), err)
	}

	noStagingLeftovers(t, dst)
}

// TestMoveFolderDstInsideSrc must refuse instead of recursing into the copy.
func TestMoveFolderDstInsideSrc(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")

	if err := os.MkdirAll(src, 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(src, "a.txt"), "a")

	err := MoveFolder(src, filepath.Join(src, "sub", "moved"), MoveFileOptions{})
	if !errors.Is(err, errDstInsideSrc) {
		t.Fatalf("err = %v; want errDstInsideSrc", err)
	}

	if got := readTestFile(t, filepath.Join(src, "a.txt")); got != "a" {
		t.Fatalf("source content = %q", got)
	}

	// A sibling whose name merely starts with the source name is fine.
	if err := MoveFolder(src, src+"-moved", MoveFileOptions{}); err != nil {
		t.Fatalf("sibling move failed: %v", err)
	}
}

// TestMoveFolderSkipsSymlink: symlinks are not followed; the move reports
// an incomplete result and keeps the source.
func TestMoveFolderSkipsSymlink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	dst := filepath.Join(dir, "moved")

	if err := os.MkdirAll(src, 0o777); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(src, "a.txt"), "a")

	if err := os.Symlink("/etc/hostname", filepath.Join(src, "link")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	forceCopyFallback(t, dst)

	err := MoveFolder(src, dst, MoveFileOptions{})
	if !errors.Is(err, errFolderIncomplete) {
		t.Fatalf("err = %v; want errFolderIncomplete", err)
	}

	if !CheckFileExist(filepath.Join(src, "a.txt")) {
		t.Fatal("source removed although a symlink was skipped")
	}

	if _, err := os.Lstat(filepath.Join(dst, "link")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink was copied/followed: %v", err)
	}
}

// TestRemoveFile covers missing files and dangling symlinks.
func TestRemoveFile(t *testing.T) {
	dir := t.TempDir()

	removed, err := RemoveFile(filepath.Join(dir, "missing.mkv"))
	if removed || err != nil {
		t.Fatalf("missing file: removed=%v err=%v; want false, nil", removed, err)
	}

	link := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "nowhere"), link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	removed, err = RemoveFile(link)
	if !removed || err != nil {
		t.Fatalf("dangling symlink: removed=%v err=%v; want true, nil", removed, err)
	}

	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dangling symlink still exists")
	}
}

// TestCheckFileExistErrors: only a successful Stat counts as existing.
func TestCheckFileExistErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	writeTestFile(t, file, "x")

	if !CheckFileExist(file) {
		t.Fatal("existing file reported missing")
	}

	if CheckFileExist(filepath.Join(file, "child")) {
		t.Fatal("path below a regular file (ENOTDIR) reported as existing")
	}

	if CheckFileExist(filepath.Join(dir, "missing")) {
		t.Fatal("missing file reported as existing")
	}
}

// TestMkdirAllModeEveryLevel: every created level gets the mode, existing
// parents are left alone.
func TestMkdirAllModeEveryLevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "a", "b", "c")
	if err := mkdirAllMode(target, 0o750); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{
		filepath.Join(dir, "a"),
		filepath.Join(dir, "a", "b"),
		target,
	} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}

		if fi.Mode().Perm() != 0o750 {
			t.Fatalf("%s mode = %v; want 0750", p, fi.Mode().Perm())
		}
	}

	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o755 {
		t.Fatalf("existing parent mode changed to %v", fi.Mode().Perm())
	}
}
