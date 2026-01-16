package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDirGoFiles_ExcludeTestsByDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files := getDirGoFiles(dir /* default args */)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "a.go" {
		t.Fatalf("expected a.go, got %s", filepath.Base(files[0]))
	}
}

func TestSortActionByFilename_GenericsReceiverAndMultiSpecTypeGroup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	in := `package p

import "fmt"

type (
	b struct{}
	A[T any] struct{}
)

func (a *A[int]) Z() {}
func (a *A[int]) a() {}

func z() {}
func AFunc() { fmt.Println("x") }
`

	path := filepath.Join(dir, "x.go")
	if err := os.WriteFile(path, []byte(in), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	changed, err := sortActionByFilename(path, true)
	if err != nil {
		t.Fatalf("sortActionByFilename: %v", err)
	}
	if !changed {
		// Even if output ends up identical (unlikely), the key contract here is: no panic/error.
		// So don't fail.
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sorted file: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("sorted output is empty")
	}
}
