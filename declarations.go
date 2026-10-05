package main

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

type declarationIndex struct {
	imports   []ast.Decl
	entries   []ast.Decl
	groups    map[token.Token][]namedDeclaration
	types     map[string]*ast.GenDecl
	methods   map[string][]namedDeclaration
	functions []namedDeclaration
}

type namedDeclaration struct {
	name string
	node ast.Decl
}

func compareNames(a, b string) int {
	aExported, bExported := isExportedName(a), isExportedName(b)
	if aExported != bExported {
		if aExported {
			return -1
		}
		return 1
	}
	if order := strings.Compare(strings.ToLower(a), strings.ToLower(b)); order != 0 {
		return order
	}
	return strings.Compare(a, b)
}

func getFuncReceiverTypeName(decl ast.Decl) string {
	function, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return ""
	}

	receiverType := function.Recv.List[0].Type
	switch t := receiverType.(type) {
	case *ast.StarExpr:
		return typeNameFromExpr(t.X)
	default:
		return typeNameFromExpr(t)
	}
}

func groupName(group *ast.GenDecl) string {
	if len(group.Specs) == 0 {
		return ""
	}
	switch group.Tok {
	case token.CONST, token.VAR:
		value, valueOK := group.Specs[0].(*ast.ValueSpec)
		if valueOK && len(value.Names) > 0 && value.Names[0] != nil {
			return value.Names[0].Name
		}
	case token.TYPE:
		typeSpec, typeOK := group.Specs[0].(*ast.TypeSpec)
		if typeOK && typeSpec.Name != nil {
			return typeSpec.Name.Name
		}
	default:
	}
	return ""
}

func indexDeclarations(file *ast.File) declarationIndex {
	index := declarationIndex{
		groups:  make(map[token.Token][]namedDeclaration),
		types:   make(map[string]*ast.GenDecl),
		methods: make(map[string][]namedDeclaration),
	}
	for _, declaration := range file.Decls {
		group, groupOK := declaration.(*ast.GenDecl)
		if !groupOK {
			continue
		}
		if group.Tok == token.IMPORT {
			index.imports = append(index.imports, group)
			continue
		}
		name := groupName(group)
		if name != "" {
			index.groups[group.Tok] = append(index.groups[group.Tok], namedDeclaration{name, group})
		}
		if group.Tok != token.TYPE {
			continue
		}
		for _, spec := range group.Specs {
			typeSpec, typeOK := spec.(*ast.TypeSpec)
			if typeOK && typeSpec.Name != nil {
				index.types[typeSpec.Name.Name] = group
			}
		}
	}
	for _, declaration := range file.Decls {
		function, functionOK := declaration.(*ast.FuncDecl)
		if !functionOK || function.Name == nil {
			continue
		}
		receiver := getFuncReceiverTypeName(function)
		if function.Recv != nil && index.types[receiver] != nil {
			index.methods[receiver] = append(index.methods[receiver], namedDeclaration{function.Name.Name, function})
			continue
		}
		if function.Name.Name == "main" || function.Name.Name == "init" {
			if function.Recv == nil {
				index.entries = append(index.entries, function)
			}
			continue
		}
		index.functions = append(index.functions, namedDeclaration{function.Name.Name, function})
	}
	return index
}

func isExportedName(name string) bool {
	if name == "" {
		return false
	}
	r := rune(name[0])
	return 'A' <= r && r <= 'Z'
}

func sortDeclarations(declarations []namedDeclaration) {
	slices.SortStableFunc(declarations, func(a, b namedDeclaration) int { return compareNames(a.name, b.name) })
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
