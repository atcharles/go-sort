package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
)

func sortActionByFilename(filename string, write bool) (bool, error) {
	content, readErr := os.ReadFile(filename) // #nosec G304 -- CLI 明确指定待排序源码，允许读取该路径。
	if readErr != nil {
		return false, fmt.Errorf("read %s: %w", filename, readErr)
	}
	output, sortErr := sortSource(filename, content)
	if sortErr != nil {
		return false, sortErr
	}
	changed := !bytes.Equal(content, output)
	if write {
		if writeErr := os.WriteFile(filename, output, 0o644); writeErr != nil { // #nosec G306 -- 排序已有源码，保留原有权限与既有写入行为。
			return false, fmt.Errorf("write %s: %w", filename, writeErr)
		}
	}
	return changed, nil
}

func sortSource(filename string, content []byte) ([]byte, error) {
	fileSet := token.NewFileSet()
	file, parseErr := parser.ParseFile(fileSet, filename, content, parser.ParseComments|parser.SkipObjectResolution)
	if parseErr != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, parseErr)
	}
	ast.SortImports(fileSet, file)
	var buffer bytes.Buffer
	writePackage(&buffer, fileSet, file, content)
	writeFileDeclarations(&buffer, file, content)
	output, formatErr := format.Source(buffer.Bytes())
	if formatErr != nil {
		return nil, fmt.Errorf("format %s: %w", filename, formatErr)
	}
	return output, nil
}
