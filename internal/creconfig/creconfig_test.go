package creconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := DirPath()
	if err != nil {
		t.Fatalf("DirPath() error: %v", err)
	}
	want := filepath.Join(home, Dir)
	if got != want {
		t.Fatalf("DirPath() = %q, want %q", got, want)
	}
}

func TestFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := FilePath("context.yaml")
	if err != nil {
		t.Fatalf("FilePath() error: %v", err)
	}
	want := filepath.Join(home, Dir, "context.yaml")
	if got != want {
		t.Fatalf("FilePath() = %q, want %q", got, want)
	}
}

func TestJoinPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := JoinPath("template-cache", "list.json")
	if err != nil {
		t.Fatalf("JoinPath() error: %v", err)
	}
	want := filepath.Join(home, Dir, "template-cache", "list.json")
	if got != want {
		t.Fatalf("JoinPath() = %q, want %q", got, want)
	}
}

func TestFilePathHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := FilePathHint("context.yaml")
	want := filepath.Join(home, Dir, "context.yaml")
	if got != want {
		t.Fatalf("FilePathHint() = %q, want %q", got, want)
	}
}

func TestFilePathHint_FallsBackToRelPath(t *testing.T) {
	t.Setenv("HOME", "")

	got := FilePathHint("context.yaml")
	want := filepath.Join(Dir, "context.yaml")
	if got != want {
		t.Fatalf("FilePathHint() = %q, want %q", got, want)
	}
}

func TestEnsureDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir() error: %v", err)
	}
	want := filepath.Join(home, Dir)
	if dir != want {
		t.Fatalf("EnsureDir() = %q, want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
	if info.Mode().Perm()&0o777 != 0o700 {
		t.Fatalf("dir mode = %o, want 0700", info.Mode().Perm()&0o777)
	}
}
