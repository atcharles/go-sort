// go-sort 按声明规则排序 Go 源文件。
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 声明按分组排序，导出名称优先，同组按名称排序。

//go:generate go mod tidy
//go:generate go install -v -trimpath -ldflags "-s -w" go-sort.go
func main() {
	logger := log.New(os.Stderr, "", 0)
	cfg, parseErr := parseFlags(os.Args[1:], os.Stderr)
	if errors.Is(parseErr, flag.ErrHelp) {
		return
	}
	if parseErr != nil {
		logger.Fatal(parseErr)
	}
	if sortErr := sortFile(cfg); sortErr != nil {
		logger.Fatal(sortErr)
	}
}

type config struct {
	path         string
	recursive    bool
	includeTests bool
	write        bool
}

type fileOptions struct {
	includeTests bool
	recursive    bool
}

type letterDecl struct {
	// Letter 是用于排序的原始声明名。
	Letter string
	// Decl 是保持原始内容的声明节点。
	Decl ast.Decl
}

type letterDeclList []letterDecl

// Len 返回声明数。
func (l letterDeclList) Len() int { return len(l) }

// Less 比较导出性和声明名。
func (l letterDeclList) Less(i, j int) bool {
	// 导出名称优先，再忽略大小写比较，最后比较原始名称。
	a, b := l[i].Letter, l[j].Letter
	ai, bi := isExportedName(a), isExportedName(b)
	if ai != bi {
		return ai
	}
	al, bl := strings.ToLower(a), strings.ToLower(b)
	if al != bl {
		return al < bl
	}
	return a < b
}

// Swap 交换声明位置。
func (l letterDeclList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func getDirGoFiles(dir string, options fileOptions) ([]string, error) {
	if dir == "./..." || dir == "./" || dir == "." || dir == "" {
		dir = "."
	}
	dir = filepath.Clean(dir)
	var files []string
	walkFn := func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
			}
			if !options.recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !options.includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		absolute, absErr := filepath.Abs(path)
		if absErr != nil {
			return fmt.Errorf("resolve path %s: %w", path, absErr)
		}
		files = append(files, absolute)
		return nil
	}
	if walkErr := filepath.Walk(dir, walkFn); walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, walkErr)
	}
	return files, nil
}

func getFuncReceiverTypeName(decl ast.Decl) string {
	fnDecl, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if fnDecl.Recv == nil || len(fnDecl.Recv.List) == 0 {
		return ""
	}

	revType := fnDecl.Recv.List[0].Type
	switch t := revType.(type) {
	case *ast.StarExpr:
		return typeNameFromExpr(t.X)
	default:
		return typeNameFromExpr(t)
	}
}

func getTypeFromFile(f *ast.File, name string) ast.Decl {
	if name == "" {
		return nil
	}
	for _, decl := range f.Decls {
		declaration, declarationOK := decl.(*ast.GenDecl)
		if !declarationOK {
			continue
		}
		if declaration.Tok != token.TYPE {
			continue
		}
		for _, spec := range declaration.Specs {
			ts, typeOK := spec.(*ast.TypeSpec)
			if !typeOK {
				continue
			}
			if ts.Name != nil && ts.Name.Name == name {
				return declaration
			}
		}
	}
	return nil
}

func isBeforePackageComment(f *ast.File, commentGroup *ast.CommentGroup) bool {
	return commentGroup.Pos() < f.Package
}

func isDeclComment(f *ast.File, commentGroup *ast.CommentGroup) bool {
	for _, decl := range f.Decls {
		if commentGroup.End()+1 == decl.Pos() {
			return true
		}
	}
	return false
}

func isExportedName(name string) bool {
	if name == "" {
		return false
	}
	r := rune(name[0])
	return 'A' <= r && r <= 'Z'
}

func isStatementComment(f *ast.File, commentGroup *ast.CommentGroup) bool {
	for _, decl := range f.Decls {
		if decl.Pos() < commentGroup.Pos() && commentGroup.End() < decl.End() {
			return true
		}
	}
	return false
}

func loadFile(cfg config) string {
	path := cfg.path
	execPath, _ := os.Executable()
	if strings.HasSuffix(execPath, path) {
		path = "."
	}
	_, err := os.Stat(path)
	if err != nil {
		log.Fatalf("file/dir %s not found\n", path)
	}
	return path
}

func parseFlags(args []string, output io.Writer) (config, error) {
	cfg := config{path: "."}
	flags := flag.NewFlagSet("go-sort", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&cfg.recursive, "r", true, "recurse into subdirectories")
	flags.BoolVar(&cfg.includeTests, "tests", false, "include *_test.go files")
	flags.BoolVar(&cfg.write, "w", true, "write result back to file")
	if parseErr := flags.Parse(args); parseErr != nil {
		return config{}, fmt.Errorf("parse flags: %w", parseErr)
	}
	positional := flags.Args()
	if len(positional) > 0 && positional[0] == "test" {
		cfg.includeTests = true
		positional = positional[1:]
	}
	if len(positional) > 0 {
		cfg.path = positional[len(positional)-1]
	}
	return cfg, nil
}

func sortActionByFilename(filename string, write bool) (changed bool, err error) {
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, filename, nil, parser.ParseComments)
	if err != nil {
		return false, err
	}
	ast.SortImports(fSet, f)
	content, err := os.ReadFile(filename) // #nosec G304 -- CLI 明确指定待排序源码，允许读取该路径。
	if err != nil {
		return false, err
	}
	var buf = new(bytes.Buffer)
	writePkg(buf, fSet, f, content)
	if err = write2buf(buf, f, content); err != nil {
		return false, err
	}
	out := buf.Bytes()
	changed = !bytes.Equal(content, out)
	if write {
		if err = os.WriteFile(filename, out, 0o644); err != nil { // #nosec G306 -- 排序已有源码，保留原有权限与既有写入行为。
			return false, err
		}
	}
	return changed, nil
}

func sortFile(cfg config) (err error) {
	files, walkErr := getDirGoFiles(loadFile(cfg), fileOptions{includeTests: cfg.includeTests, recursive: cfg.recursive})
	if walkErr != nil {
		return walkErr
	}
	for _, file := range files {
		_, err = sortActionByFilename(file, cfg.write)
		if err != nil {
			return fmt.Errorf("sort file %s error: %w", file, err)
		}
	}
	return nil
}

func typeNameFromExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // T[P]
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.IndexListExpr: // T[A, B]
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	default:
	}
	return ""
}

func write2buf(buf *bytes.Buffer, f *ast.File, content []byte) (err error) {
	write2bufTop(buf, f, content)
	write2bufTopComment(buf, f, content)
	writeMain(buf, f, content)
	write2bufGenDecl(buf, f, content, token.CONST, false)
	buf.WriteString("\n")
	write2bufGenDecl(buf, f, content, token.VAR, false)
	buf.WriteString("\n")
	write2bufGenDecl(buf, f, content, token.TYPE, true)
	write2bufFunc(buf, f, content, true)
	ret, err := format.Source(buf.Bytes())
	if err != nil {
		return
	}
	buf.Reset()
	buf.Write(ret)
	return
}

func write2bufAsDecl(buf *bytes.Buffer, content []byte, decl ast.Decl, writeLine bool) {
	declaration := decl.(*ast.GenDecl)
	posStart := declaration.Pos() - 1
	if declaration.Doc != nil {
		posStart = declaration.Doc.Pos() - 1
	}
	buf.Write(content[posStart:declaration.End()])
	if writeLine {
		buf.WriteString("\n")
	}
}

func write2bufAsFunc(buf *bytes.Buffer, content []byte, decl ast.Decl, writeLine bool) {
	declaration := decl.(*ast.FuncDecl)
	posStart := declaration.Pos() - 1
	if declaration.Doc != nil {
		posStart = declaration.Doc.Pos() - 1
	}
	buf.Write(content[posStart:declaration.End()])
	if writeLine {
		buf.WriteString("\n")
	}
}

func write2bufFunc(buf *bytes.Buffer, f *ast.File, content []byte, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		declaration, declarationOK := decl.(*ast.FuncDecl)
		if !declarationOK {
			continue
		}
		// main 与 init 在文件前部单独写出。
		if declaration.Name != nil && (declaration.Name.Name == "main" || declaration.Name.Name == "init") {
			continue
		}
		// 同文件类型的接收者方法随类型写出。
		if declaration.Recv != nil {
			if getTypeFromFile(f, getFuncReceiverTypeName(declaration)) != nil {
				continue
			}
		}
		if declaration.Name == nil {
			continue
		}
		list = append(list, letterDecl{Letter: declaration.Name.Name, Decl: declaration})
	}
	sort.Stable(list)
	for _, node := range list {
		write2bufAsFunc(buf, content, node.Decl, writeLine)
	}
}

func write2bufGenDecl(buf *bytes.Buffer, f *ast.File, content []byte, tk token.Token, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		declaration, declarationOK := decl.(*ast.GenDecl)
		if !declarationOK {
			continue
		}
		if declaration.Tok != tk || declaration.Tok == token.IMPORT {
			continue
		}
		// 多 spec 声明按首项排序，保持整个声明组不拆分。
		switch tk {
		case token.CONST, token.VAR:
			if len(declaration.Specs) == 0 {
				continue
			}
			vs, valueOK := declaration.Specs[0].(*ast.ValueSpec)
			if !valueOK || len(vs.Names) == 0 || vs.Names[0] == nil {
				continue
			}
			list = append(list, letterDecl{Letter: vs.Names[0].Name, Decl: declaration})
		case token.TYPE:
			if len(declaration.Specs) == 0 {
				continue
			}
			ts, typeOK := declaration.Specs[0].(*ast.TypeSpec)
			if !typeOK || ts.Name == nil {
				continue
			}
			list = append(list, letterDecl{Letter: ts.Name.Name, Decl: declaration})
		default:
		}
	}
	sort.Stable(list)
	for _, node := range list {
		write2bufAsDecl(buf, content, node.Decl, writeLine)
		declaration := node.Decl.(*ast.GenDecl)
		if declaration.Tok == token.TYPE {
			// 按类型组内原顺序写出各类型的接收者方法。
			for _, spec := range declaration.Specs {
				ts, typeOK := spec.(*ast.TypeSpec)
				if !typeOK || ts.Name == nil {
					continue
				}
				writeTypesReceiverFunc(f, ts.Name.Name, buf, content, writeLine)
			}
		}
	}
}

func write2bufTop(buf *bytes.Buffer, f *ast.File, content []byte) {
	list := make(letterDeclList, 0)
	for _, decl := range f.Decls {
		if declaration, declarationOK := decl.(*ast.GenDecl); declarationOK {
			if declaration.Tok == token.IMPORT {
				list = append(list, letterDecl{Letter: "import", Decl: declaration})
			}
		}
	}
	for _, decl := range list {
		write2bufAsDecl(buf, content, decl.Decl, true)
	}
}

func write2bufTopComment(buf *bytes.Buffer, f *ast.File, content []byte) {
	for _, commentGroup := range f.Comments {
		if !isDeclComment(f, commentGroup) &&
			!isStatementComment(f, commentGroup) &&
			!isBeforePackageComment(f, commentGroup) {
			buf.Write(content[commentGroup.Pos()-1 : commentGroup.End()])
			buf.WriteString("\n")
		}
	}
}

func writeMain(buf *bytes.Buffer, f *ast.File, content []byte) {
	for _, decl := range f.Decls {
		declaration, declarationOK := decl.(*ast.FuncDecl)
		if !declarationOK {
			continue
		}
		// 接收者方法不作为入口函数写出。
		if declaration.Recv != nil {
			continue
		}
		if declaration.Name.Name == "main" || declaration.Name.Name == "init" {
			write2bufAsFunc(buf, content, declaration, true)
		}
	}
}

func writePkg(buf *bytes.Buffer, fSet *token.FileSet, f *ast.File, content []byte) {
	line := fSet.Position(f.Package).Line
	var bufTop = make([]byte, 0)
	var idx = 0
	for range line {
		c := bytes.IndexByte(content[idx:], '\n')
		if c == -1 {
			break
		}
		idx += c + 1
	}
	bufTop = append(bufTop, content[:idx]...)
	buf.Write(bufTop)
}

// writeTypesReceiverFunc 写出指定类型的接收者方法。
func writeTypesReceiverFunc(f *ast.File, name string, buf *bytes.Buffer, content []byte, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		declaration, declarationOK := decl.(*ast.FuncDecl)
		if !declarationOK {
			continue
		}
		if declaration.Recv == nil {
			continue
		}
		if getFuncReceiverTypeName(declaration) != name {
			continue
		}
		if declaration.Name == nil {
			continue
		}
		list = append(list, letterDecl{Letter: declaration.Name.Name, Decl: declaration})
	}
	sort.Stable(list)
	for _, node := range list {
		write2bufAsFunc(buf, content, node.Decl, writeLine)
	}
}
