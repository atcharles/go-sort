// this file is used to sort go file
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// sort a go file,
// sort constants, variables, functions, structs, interfaces in order of their appearance in the file
// export units are on top, non-export units are below
// sort units by FirstLetter ASC

// the following is the code to generate the sort executable file

//go:generate go mod tidy
//go:generate go install -v -trimpath -ldflags "-s -w" go-sort.go
func main() {
	log.SetFlags(0)
	cfg := parseFlags()
	if e := sortFile(cfg); e != nil {
		log.Fatalln(e)
	}
}

type config struct {
	path         string
	recursive    bool
	includeTests bool
	write        bool
}

type letterDecl struct {
	Letter string
	Decl   ast.Decl
}

type letterDeclList []letterDecl

func (l letterDeclList) Len() int { return len(l) }

func (l letterDeclList) Less(i, j int) bool {
	// Deterministic ordering:
	// 1) exported (upper-case) first
	// 2) case-insensitive letter order
	// 3) original string compare as tie-breaker
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

func (l letterDeclList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func getDirGoFiles(dir string, args ...any) []string {
	if dir == "./..." || dir == "./" || dir == "." || dir == "" {
		dir = "."
	}
	useTest := false
	recursive := true
	for _, arg := range args {
		switch _arg := arg.(type) {
		case bool:
			// Back-compat: first bool is included tests, second bool is recursive.
			if !useTest {
				useTest = _arg
			} else {
				recursive = _arg
			}
		}
	}
	var files []string
	walkFn := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !useTest && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		abs, e := filepath.Abs(path)
		if e != nil {
			return e
		}
		files = append(files, abs)
		return nil
	}
	_ = filepath.Walk(dir, walkFn)
	return files
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
		_decl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if _decl.Tok != token.TYPE {
			continue
		}
		for _, spec := range _decl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name != nil && ts.Name.Name == name {
				return _decl
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

func parseFlags() config {
	var cfg config
	flag.BoolVar(&cfg.recursive, "r", true, "recurse into subdirectories")
	flag.BoolVar(&cfg.includeTests, "tests", false, "include *_test.go files")
	flag.BoolVar(&cfg.write, "w", true, "write result back to file")
	flag.Parse()

	// Default path: last arg if present, else current dir.
	args := flag.Args()
	if len(args) == 0 {
		cfg.path = "."
	} else {
		cfg.path = args[len(args)-1]
	}
	return cfg
}

func sortActionByFilename(filename string, write bool) (changed bool, err error) {
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, filename, nil, parser.ParseComments)
	if err != nil {
		return false, err
	}
	ast.SortImports(fSet, f)
	content, err := os.ReadFile(filename)
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
		if err = os.WriteFile(filename, out, 0644); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func sortFile(cfg config) (err error) {
	for _, file := range getDirGoFiles(loadFile(cfg), cfg.includeTests, cfg.recursive) {
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
	_decl := decl.(*ast.GenDecl)
	posStart := _decl.Pos() - 1
	if _decl.Doc != nil {
		posStart = _decl.Doc.Pos() - 1
	}
	buf.Write(content[posStart:_decl.End()])
	if writeLine {
		buf.WriteString("\n")
	}
}

func write2bufAsFunc(buf *bytes.Buffer, content []byte, decl ast.Decl, writeLine bool) {
	_decl := decl.(*ast.FuncDecl)
	posStart := _decl.Pos() - 1
	if _decl.Doc != nil {
		posStart = _decl.Doc.Pos() - 1
	}
	buf.Write(content[posStart:_decl.End()])
	if writeLine {
		buf.WriteString("\n")
	}
}

func write2bufFunc(buf *bytes.Buffer, f *ast.File, content []byte, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		_decl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		//if main or init, skip
		if _decl.Name != nil && (_decl.Name.Name == "main" || _decl.Name.Name == "init") {
			continue
		}
		//if is a receiver function, and the receiver type is in the same file, skip
		if _decl.Recv != nil {
			if getTypeFromFile(f, getFuncReceiverTypeName(_decl)) != nil {
				continue
			}
		}
		if _decl.Name == nil {
			continue
		}
		list = append(list, letterDecl{Letter: _decl.Name.Name, Decl: _decl})
	}
	sort.Stable(list)
	for _, node := range list {
		write2bufAsFunc(buf, content, node.Decl, writeLine)
	}
}

func write2bufGenDecl(buf *bytes.Buffer, f *ast.File, content []byte, tk token.Token, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		_decl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if _decl.Tok != tk || _decl.Tok == token.IMPORT {
			continue
		}
		// If a decl has multiple specs, we still sort by the first spec's name.
		// (We keep the decl group intact to avoid surprising rewrites.)
		switch tk {
		case token.CONST, token.VAR:
			if len(_decl.Specs) == 0 {
				continue
			}
			vs, ok := _decl.Specs[0].(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 || vs.Names[0] == nil {
				continue
			}
			list = append(list, letterDecl{Letter: vs.Names[0].Name, Decl: _decl})
		case token.TYPE:
			if len(_decl.Specs) == 0 {
				continue
			}
			ts, ok := _decl.Specs[0].(*ast.TypeSpec)
			if !ok || ts.Name == nil {
				continue
			}
			list = append(list, letterDecl{Letter: ts.Name.Name, Decl: _decl})
		default:
		}
	}
	sort.Stable(list)
	for _, node := range list {
		write2bufAsDecl(buf, content, node.Decl, writeLine)
		_decl := node.Decl.(*ast.GenDecl)
		if _decl.Tok == token.TYPE {
			//get the group of types, and write receiver function
			for _, spec := range _decl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
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
		if _decl, ok := decl.(*ast.GenDecl); ok {
			if _decl.Tok == token.IMPORT {
				list = append(list, letterDecl{Letter: "import", Decl: _decl})
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
		_decl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// if it has receiver, skip
		if _decl.Recv != nil {
			continue
		}
		if _decl.Name.Name == "main" || _decl.Name.Name == "init" {
			write2bufAsFunc(buf, content, _decl, true)
		}
	}
}

func writePkg(buf *bytes.Buffer, fSet *token.FileSet, f *ast.File, content []byte) {
	line := fSet.Position(f.Package).Line
	var bufTop = make([]byte, 0)
	var idx = 0
	for i := 0; i < line; i++ {
		c := bytes.IndexByte(content[idx:], '\n')
		if c == -1 {
			break
		}
		idx += c + 1
	}
	bufTop = append(bufTop, content[:idx]...)
	buf.Write(bufTop)
}

// writeTypesReceiverFunc write receiver function of type
func writeTypesReceiverFunc(f *ast.File, name string, buf *bytes.Buffer, content []byte, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		_decl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if _decl.Recv == nil {
			continue
		}
		if getFuncReceiverTypeName(_decl) != name {
			continue
		}
		if _decl.Name == nil {
			continue
		}
		list = append(list, letterDecl{Letter: _decl.Name.Name, Decl: _decl})
	}
	sort.Stable(list)
	for _, node := range list {
		write2bufAsFunc(buf, content, node.Decl, writeLine)
	}
}
