package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestDeclarationOrder 固定大小写与稳定排序规则。
func TestDeclarationOrder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{"exported", []string{"z", "B", "a", "A"}, []string{"A", "B", "a", "z"}},
		{"case_tie", []string{"alpha", "aLPHA", "Alpha", "ALPHA"}, []string{"ALPHA", "Alpha", "aLPHA", "alpha"}},
		{"empty", []string{"z", "", "A"}, []string{"A", "", "z"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := make(letterDeclList, 0, len(tc.input))
			for _, name := range tc.input {
				list = append(list, letterDecl{Letter: name})
			}
			sort.Stable(list)
			got := make([]string, 0, len(list))
			for _, entry := range list {
				got = append(got, entry.Letter)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("排序差异 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestReceiverTypeName 覆盖普通、指针及单/多参数泛型接收者。
func TestReceiverTypeName(t *testing.T) {
	t.Parallel()
	cases := []struct{ source, want string }{
		{"func F() {}", ""},
		{"func (x Thing) F() {}", "Thing"},
		{"func (x *Thing) F() {}", "Thing"},
		{"func (x Box[T]) F() {}", "Box"},
		{"func (x *Box[T]) F() {}", "Box"},
		{"func (x Pair[A, B]) F() {}", "Pair"},
		{"func (x *Pair[A, B]) F() {}", "Pair"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			file, parseErr := parser.ParseFile(token.NewFileSet(), "receiver.go", "package sample\n"+tc.source, 0)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if diff := cmp.Diff(tc.want, getFuncReceiverTypeName(file.Decls[0])); diff != "" {
				t.Fatal(diff)
			}
		})
	}
	if got := getFuncReceiverTypeName(&ast.GenDecl{}); got != "" {
		t.Fatalf("非函数接收者: %s", got)
	}
}

// TestTypeLookup 固定多 spec 声明中的类型查找行为。
func TestTypeLookup(t *testing.T) {
	t.Parallel()
	file, parseErr := parser.ParseFile(token.NewFileSet(), "types.go", "package sample\nvar V int\ntype (A struct{}; B[T any] struct{})", 0)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	for _, name := range []string{"A", "B"} {
		if got := getTypeFromFile(file, name); got != file.Decls[1] {
			t.Fatalf("未找到分组类型 %s", name)
		}
	}
	for _, name := range []string{"", "Missing"} {
		if got := getTypeFromFile(file, name); got != nil {
			t.Fatalf("意外找到 %s", name)
		}
	}
}

// TestGoldenSort 用 main bbf5a84 的输出验证排序、注释与包行保持兼容。
func TestGoldenSort(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"declarations", "generics", "comments", "package"} {
		t.Run(name, func(t *testing.T) {
			input := readTestFile(t, filepath.Join("testdata", name+".input"))
			want := readTestFile(t, filepath.Join("testdata", name+".golden"))
			path := filepath.Join(t.TempDir(), "sample.go")
			writeTestFile(t, path, input)
			_, dryErr := sortActionByFilename(path, false)
			if dryErr != nil {
				t.Fatal(dryErr)
			}
			if diff := cmp.Diff(input, readTestFile(t, path)); diff != "" {
				t.Fatalf("-w=false 修改文件:\n%s", diff)
			}
			_, writeErr := sortActionByFilename(path, true)
			if writeErr != nil {
				t.Fatal(writeErr)
			}
			if diff := cmp.Diff(string(want), string(readTestFile(t, path))); diff != "" {
				t.Fatalf("黄金样例差异 (-want +got):\n%s", diff)
			}
			changed, repeatErr := sortActionByFilename(path, true)
			if repeatErr != nil {
				t.Fatal(repeatErr)
			}
			if changed {
				t.Fatal("重复排序改变输出")
			}
		})
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return data
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
}
