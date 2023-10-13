// this file is used to sort go file
package main

import (
	"bytes"
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
	if e := sortFile(); e != nil {
		log.Fatalln(e)
	}
}

// letterDecl is a letter and its declaration
type letterDecl struct {
	Letter string
	Decl   ast.Decl
}

// letterDeclList is a list of letterDecl
type letterDeclList []letterDecl

func (l letterDeclList) Len() int { return len(l) }

func (l letterDeclList) Less(i, j int) bool { return l[i].Letter < l[j].Letter }

func (l letterDeclList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func getDirGoFiles(dir string, args ...any) []string {
	if dir == "./..." || dir == "./" || dir == "." || dir == "" {
		dir = "."
	}
	useTest := false
	for _, arg := range args {
		switch _arg := arg.(type) {
		case bool:
			useTest = _arg
		}
	}
	var files []string
	_ = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil ||
			info.IsDir() ||
			!strings.HasSuffix(path, ".go") ||
			(!useTest && strings.Contains(path, "_test.go")) {
			return nil
		}
		path, e := filepath.Abs(path)
		if e != nil {
			return e
		}
		files = append(files, path)
		return nil
	})
	return files
}

func getFuncReceiverTypeName(decl ast.Decl) string {
	fnDecl, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if fnDecl.Recv == nil {
		return ""
	}
	_type := fnDecl.Recv.List[0].Type
	var _typeName string
	switch __t := _type.(type) {
	case *ast.StarExpr:
		switch _type1 := __t.X.(type) {
		case *ast.Ident:
			_typeName = _type1.Name
		case *ast.IndexExpr:
			//Generics type
			_typeName = _type1.X.(*ast.Ident).Name
		default:
			fmt.Printf("unknown type: %T, %#+v\n", _type1, __t.X)
		}
	case *ast.Ident:
		_typeName = __t.Name
	}
	return _typeName
}

func getTypeFromFile(f *ast.File, name string) ast.Decl {
	for _, decl := range f.Decls {
		_decl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if _decl.Tok != token.TYPE {
			continue
		}
		for _, spec := range _decl.Specs {
			_type := spec.(*ast.TypeSpec)
			if _type.Name.Name == name {
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

func isStatementComment(f *ast.File, commentGroup *ast.CommentGroup) bool {
	for _, decl := range f.Decls {
		if decl.Pos() < commentGroup.Pos() && commentGroup.End() < decl.End() {
			return true
		}
	}
	return false
}

func loadFile() string {
	path := os.Args[len(os.Args)-1]
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

func sortActionByFilename(filename string) (err error) {
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, filename, nil, parser.ParseComments)
	if err != nil {
		return
	}
	ast.SortImports(fSet, f)
	content, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	var buf = new(bytes.Buffer)
	writePkg(buf, fSet, f, content)
	if err = write2buf(buf, f, content); err != nil {
		return
	}
	if err = os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		return
	}
	return
}

func sortFile() (err error) {
	for _, file := range getDirGoFiles(loadFile()) {
		if err = sortActionByFilename(file); err != nil {
			return fmt.Errorf("sort file %s error: %w", file, err)
		}
	}
	return
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
		if _decl.Name.Name == "main" || _decl.Name.Name == "init" {
			continue
		}
		//if is a receiver function, and the receiver type is in the same file, skip
		if _decl.Recv != nil {
			if getTypeFromFile(f, getFuncReceiverTypeName(_decl)) != nil {
				continue
			}
		}
		list = append(list, letterDecl{Letter: _decl.Name.Name, Decl: _decl})
	}
	sort.Sort(list)
	for _, node := range list {
		write2bufAsFunc(buf, content, node.Decl, writeLine)
	}
}

func write2bufGenDecl(buf *bytes.Buffer, f *ast.File, content []byte, tk token.Token, writeLine bool) {
	var list = make(letterDeclList, 0)
	for _, decl := range f.Decls {
		if _decl, ok := decl.(*ast.GenDecl); ok {
			if _decl.Tok == tk && _decl.Tok != token.IMPORT && _decl.Tok != token.TYPE {
				name := _decl.Specs[0].(*ast.ValueSpec).Names[0].Name
				list = append(list, letterDecl{Letter: name, Decl: _decl})
			}
			if _decl.Tok == tk && _decl.Tok == token.TYPE {
				name := _decl.Specs[0].(*ast.TypeSpec).Name.Name
				list = append(list, letterDecl{Letter: name, Decl: _decl})
			}
		}
	}
	sort.Sort(list)
	for _, node := range list {
		write2bufAsDecl(buf, content, node.Decl, writeLine)
		_decl := node.Decl.(*ast.GenDecl)
		if _decl.Tok == token.TYPE {
			//get the group of types, and write receiver function
			for _, spec := range _decl.Specs {
				__name := spec.(*ast.TypeSpec).Name.Name
				writeTypesReceiverFunc(f, __name, buf, content, writeLine)
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
		// if has receiver, skip
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
		list = append(list, letterDecl{Letter: _decl.Name.Name, Decl: _decl})
	}
	sort.Sort(list)
	for _, node := range list {
		write2bufAsFunc(buf, content, node.Decl, writeLine)
	}
}
