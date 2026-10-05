package main

import (
	"errors"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
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
			list := make([]namedDeclaration, 0, len(tc.input))
			for _, name := range tc.input {
				list = append(list, namedDeclaration{name: name})
			}
			sortDeclarations(list)
			got := make([]string, 0, len(list))
			for _, entry := range list {
				got = append(got, entry.name)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("排序差异 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDirectoryOptions 回归递归与测试文件选择的四种组合。
func TestDirectoryOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                    string
		recursive, includeTests bool
		want                    []string
	}{
		{"recursive_without_tests", true, false, []string{"a.go", "sub/b.go"}},
		{"recursive_with_tests", true, true, []string{"a.go", "a_test.go", "sub/b.go", "sub/b_test.go"}},
		{"local_without_tests", false, false, []string{"a.go"}},
		{"local_with_tests", false, true, []string{"a.go", "a_test.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, relative := range []string{"a.go", "a_test.go", "sub/b.go", "sub/b_test.go", "vendor/v.go", ".git/g.go", "notes.txt"} {
				path := filepath.Join(dir, relative)
				if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o700); mkdirErr != nil {
					t.Fatal(mkdirErr)
				}
				writeTestFile(t, path, []byte("package sample\n"))
			}
			files, walkErr := getDirGoFiles(dir, fileOptions{includeTests: tc.includeTests, recursive: tc.recursive})
			if walkErr != nil {
				t.Fatal(walkErr)
			}
			got := make([]string, 0, len(files))
			for _, path := range files {
				relative, relErr := filepath.Rel(dir, path)
				if relErr != nil {
					t.Fatal(relErr)
				}
				got = append(got, filepath.ToSlash(relative))
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("文件选择差异 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestGoldenSort 固定 main 输出及无结尾换行输入的修复后输出。
func TestGoldenSort(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"declarations", "generics", "comments", "package", "eof", "package_eof", "comment_eof", "value_eof"} {
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

// TestInputPathAliases 验证当前目录的路径别名均可通过路径校验。
func TestInputPathAliases(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"", ".", "./", "./..."} {
		resolved, resolveErr := resolveInputPath(path)
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		if filepath.Clean(resolved) != "." {
			t.Fatalf("路径 %q 解析为 %q", path, resolved)
		}
	}
}

// TestParseFlags 固定 test 别名、默认值和旧位置路径行为。
func TestParseFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want config
	}{
		{"defaults", nil, config{".", true, false, true}},
		{"path", []string{"."}, config{".", true, false, true}},
		{"tests_flag", []string{"-tests", "."}, config{".", true, true, true}},
		{"tests_alias", []string{"test", "."}, config{".", true, true, true}},
		{"alias_default_path", []string{"test"}, config{".", true, true, true}},
		{"alias_with_flags", []string{"-r=false", "-w=false", "test", "src"}, config{"src", false, true, false}},
		{"last_path", []string{"ignored", "src"}, config{"src", true, false, true}},
		{"ellipsis", []string{"./..."}, config{"./...", true, false, true}},
		{"explicit_test_path", []string{"./test"}, config{"./test", true, false, true}},
		{"flag_terminator", []string{"--", "-source"}, config{"-source", true, false, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, parseErr := parseFlags(tc.args, io.Discard)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(config{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
	_, helpErr := parseFlags([]string{"-h"}, io.Discard)
	if !errors.Is(helpErr, flag.ErrHelp) {
		t.Fatalf("help: %v", helpErr)
	}
	_, invalidErr := parseFlags([]string{"-unknown"}, io.Discard)
	if invalidErr == nil {
		t.Fatal("未知参数应报错")
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

// TestSortErrors 验证失败不会写坏文件，底层路径错误可以解包。
func TestSortErrors(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing.go")
	_, missingErr := sortActionByFilename(missing, true)
	if !errors.Is(missingErr, os.ErrNotExist) {
		t.Fatalf("缺失文件错误: %v", missingErr)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.go")
	input := []byte("package sample\nfunc Broken(\n")
	writeTestFile(t, invalid, input)
	_, parseErr := sortActionByFilename(invalid, true)
	if parseErr == nil {
		t.Fatal("非法源码应报错")
	}
	if diff := cmp.Diff(input, readTestFile(t, invalid)); diff != "" {
		t.Fatal(diff)
	}
	_, walkErr := getDirGoFiles(missing, fileOptions{recursive: true})
	if !errors.Is(walkErr, os.ErrNotExist) {
		t.Fatalf("缺失目录错误: %v", walkErr)
	}
}

// TestSortFileMissingPath 验证命令流程返回可解包错误，不在库函数内退出进程。
func TestSortFileMissingPath(t *testing.T) {
	t.Parallel()
	cfg := config{path: filepath.Join(t.TempDir(), "missing"), recursive: true, write: true}
	if sortErr := sortFile(cfg); !errors.Is(sortErr, os.ErrNotExist) {
		t.Fatalf("缺失路径错误: %v", sortErr)
	}
}

// TestTestsAliasSort 验证 test 与 -tests 对文件产生相同效果。
func TestTestsAliasSort(t *testing.T) {
	t.Parallel()
	input := readTestFile(t, filepath.Join("testdata", "declarations.input"))
	want := readTestFile(t, filepath.Join("testdata", "declarations.golden"))
	for _, args := range [][]string{{}, {"-tests"}, {"test"}} {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "sample.go")
		testPath := filepath.Join(dir, "sample_test.go")
		writeTestFile(t, sourcePath, input)
		writeTestFile(t, testPath, input)
		cfg, parseErr := parseFlags(append(args, dir), io.Discard)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if sortErr := sortFile(cfg); sortErr != nil {
			t.Fatal(sortErr)
		}
		if diff := cmp.Diff(want, readTestFile(t, sourcePath)); diff != "" {
			t.Fatal(diff)
		}
		expectedTest := input
		if len(args) > 0 {
			expectedTest = want
		}
		if diff := cmp.Diff(expectedTest, readTestFile(t, testPath)); diff != "" {
			t.Fatal(diff)
		}
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
		if got := indexDeclarations(file).types[name]; got != file.Decls[1] {
			t.Fatalf("未找到分组类型 %s", name)
		}
	}
	for _, name := range []string{"", "Missing"} {
		if got := indexDeclarations(file).types[name]; got != nil {
			t.Fatalf("意外找到 %s", name)
		}
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
