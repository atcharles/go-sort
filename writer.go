package main

import (
	"bytes"
	"go/ast"
	"go/token"
)

func isBeforePackageComment(file *ast.File, commentGroup *ast.CommentGroup) bool {
	return commentGroup.Pos() < file.Package
}

func isDeclComment(file *ast.File, commentGroup *ast.CommentGroup) bool {
	for _, decl := range file.Decls {
		if commentGroup.End()+1 == decl.Pos() {
			return true
		}
	}
	return false
}

func isStatementComment(file *ast.File, commentGroup *ast.CommentGroup) bool {
	for _, decl := range file.Decls {
		if decl.Pos() < commentGroup.Pos() && commentGroup.End() < decl.End() {
			return true
		}
	}
	return false
}

func writeDeclaration(buffer *bytes.Buffer, content []byte, declaration ast.Decl, newline bool) {
	start := declaration.Pos() - 1
	var doc *ast.CommentGroup
	switch node := declaration.(type) {
	case *ast.GenDecl:
		doc = node.Doc
	case *ast.FuncDecl:
		doc = node.Doc
	default:
	}
	if doc != nil {
		start = doc.Pos() - 1
	}
	writeSourceSpan(buffer, content, int(start), declaration.End())
	if newline {
		buffer.WriteString("\n")
	}
}

func writeDeclarationGroup(buffer *bytes.Buffer, content []byte, index declarationIndex, kind token.Token) {
	declarations := index.groups[kind]
	sortDeclarations(declarations)
	for _, declaration := range declarations {
		writeDeclaration(buffer, content, declaration.node, kind == token.TYPE)
		if kind != token.TYPE {
			continue
		}
		group := declaration.node.(*ast.GenDecl)
		for _, spec := range group.Specs {
			typeSpec, typeOK := spec.(*ast.TypeSpec)
			if !typeOK || typeSpec.Name == nil {
				continue
			}
			writeSortedDeclarations(buffer, content, index.methods[typeSpec.Name.Name], true)
		}
	}
}

func writeFileDeclarations(buffer *bytes.Buffer, file *ast.File, content []byte) {
	index := indexDeclarations(file)
	for _, declaration := range index.imports {
		writeDeclaration(buffer, content, declaration, true)
	}
	writeTopComments(buffer, file, content)
	for _, entry := range index.entries {
		writeDeclaration(buffer, content, entry, true)
	}
	writeDeclarationGroup(buffer, content, index, token.CONST)
	buffer.WriteString("\n")
	writeDeclarationGroup(buffer, content, index, token.VAR)
	buffer.WriteString("\n")
	writeDeclarationGroup(buffer, content, index, token.TYPE)
	writeSortedDeclarations(buffer, content, index.functions, true)
}

func writePackage(buffer *bytes.Buffer, fileSet *token.FileSet, file *ast.File, content []byte) {
	line := fileSet.Position(file.Package).Line
	offset := 0
	for range line {
		newlineOffset := bytes.IndexByte(content[offset:], '\n')
		if newlineOffset == -1 {
			offset = len(content)
			break
		}
		offset += newlineOffset + 1
	}
	buffer.Write(content[:offset])
}

func writeSortedDeclarations(buffer *bytes.Buffer, content []byte, declarations []namedDeclaration, newline bool) {
	sortDeclarations(declarations)
	for _, declaration := range declarations {
		writeDeclaration(buffer, content, declaration.node, newline)
	}
}

func writeSourceSpan(buffer *bytes.Buffer, content []byte, start int, end token.Pos) {
	// 保留声明后的一个字节；EOF 处以换行替代越界的容量字节。
	buffer.Write(content[start:min(int(end), len(content))])
	if int(end) > len(content) {
		buffer.WriteString("\n")
	}
}

func writeTopComments(buffer *bytes.Buffer, file *ast.File, content []byte) {
	for _, commentGroup := range file.Comments {
		if !isDeclComment(file, commentGroup) &&
			!isStatementComment(file, commentGroup) &&
			!isBeforePackageComment(file, commentGroup) {
			writeSourceSpan(buffer, content, int(commentGroup.Pos()-1), commentGroup.End())
			buffer.WriteString("\n")
		}
	}
}
