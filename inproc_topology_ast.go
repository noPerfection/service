package service

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology/config"
)

type hostPackageAST struct {
	info   *package_url.PackageInfo
	fset   *token.FileSet
	files  []*ast.File
	consts map[string]string
	funcs  map[string]*ast.FuncDecl
}

func loadHostPackage(hostModuleURL string) (*hostPackageAST, error) {
	if hostModuleURL == "" {
		return nil, fmt.Errorf("host module url is empty")
	}

	info, err := package_url.New(hostModuleURL)
	if err != nil {
		return nil, fmt.Errorf("package_url.New(%q): %w", hostModuleURL, err)
	}
	if !info.IsMain() {
		return nil, fmt.Errorf("host module %q is not a main package", hostModuleURL)
	}

	sourceFiles := info.SourceFiles()
	if len(sourceFiles) == 0 {
		return nil, fmt.Errorf("host module %q has no source files", hostModuleURL)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(sourceFiles))
	for _, sourceFile := range sourceFiles {
		file, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parser.ParseFile(%q): %w", sourceFile, err)
		}
		files = append(files, file)
	}

	host := &hostPackageAST{
		info:   info,
		fset:   fset,
		files:  files,
		consts: packageConsts(files),
		funcs:  packageFuncs(files),
	}
	return host, nil
}

func packageConsts(files []*ast.File) map[string]string {
	consts := make(map[string]string)
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valueSpec.Names {
					if name.Name == "_" {
						continue
					}
					if i >= len(valueSpec.Values) {
						continue
					}
					if value, ok := stringExpr(valueSpec.Values[i], consts); ok {
						consts[name.Name] = value
					}
				}
			}
		}
	}
	return consts
}

func packageFuncs(files []*ast.File) map[string]*ast.FuncDecl {
	funcs := make(map[string]*ast.FuncDecl)
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Recv != nil {
				continue
			}
			funcs[fn.Name.Name] = fn
		}
	}
	return funcs
}

func importLocalName(files []*ast.File, importPath string) (string, bool) {
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.IMPORT {
				continue
			}
			for _, spec := range gen.Specs {
				importSpec, ok := spec.(*ast.ImportSpec)
				if !ok || importSpec.Path == nil {
					continue
				}
				pathValue, err := strconv.Unquote(importSpec.Path.Value)
				if err != nil || pathValue != importPath {
					continue
				}
				if importSpec.Name != nil {
					return importSpec.Name.Name, true
				}
				return path.Base(pathValue), true
			}
		}
	}
	return "", false
}

func findMainFunc(files []*ast.File) *ast.FuncDecl {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "main" && fn.Recv == nil {
				return fn
			}
		}
	}
	return nil
}

const startInprocTopologyCall = "startInprocTopology()"

var hostServiceConstructors = []string{"New", "NewExt", "NewProxy"}

// HostMainSourceContains reports whether main.go contains substr, or when substr is
// startInprocTopologyCall whether main() actively calls startInprocTopology().
func HostMainSourceContains(hostModuleURL string, substr string) (bool, error) {
	host, err := loadHostPackage(hostModuleURL)
	if err != nil {
		return false, fmt.Errorf("loadHostPackage: %w", err)
	}
	return hostMainSourceContains(host, substr), nil
}

func hostMainSourceContains(host *hostPackageAST, substr string) bool {
	if host == nil {
		return false
	}
	if substr == startInprocTopologyCall {
		return hostMainReferencesStartInprocTopologyCall(host)
	}
	mainPath := filepath.Join(hostPackageDir(host), "main.go")
	content, err := os.ReadFile(mainPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), substr)
}

func hostMainReferencesStartInprocTopologyCall(host *hostPackageAST) bool {
	if host == nil {
		return false
	}
	mainFn := findMainFunc(host.files)
	return mainFn != nil && mainBodyCallsStartInprocTopology(mainFn)
}

func mainBodyCallsStartInprocTopology(mainFn *ast.FuncDecl) bool {
	if mainFn == nil || mainFn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == "startInprocTopology" {
			found = true
			return false
		}
		return true
	})
	return found
}

// EnsureStartInprocTopologyCall inserts startInprocTopology() before the host service Start() in main.go.
func EnsureStartInprocTopologyCall(hostModuleURL string, serviceName string) (mainEdited bool, err error) {
	if hostModuleURL == "" {
		return false, fmt.Errorf("host module url is empty")
	}
	if serviceName == "" {
		return false, fmt.Errorf("service name is empty")
	}

	host, err := loadHostPackage(hostModuleURL)
	if err != nil {
		return false, fmt.Errorf("loadHostPackage: %w", err)
	}

	file, mainFn := findMainFile(host)
	if mainFn == nil || mainFn.Body == nil {
		return false, fmt.Errorf("main() not found in host package")
	}
	if mainBodyCallsStartInprocTopology(mainFn) {
		return false, nil
	}
	mainPath := host.fset.File(file.Pos()).Name()

	servicePkg := serviceImportLocalName(host.files)
	appIdent, err := hostAppIdentFromServiceConstructor(mainFn, servicePkg, serviceName, host.consts)
	if err != nil {
		if errors.Is(err, ErrDynamicServiceName) {
			return false, fmt.Errorf("%w: please add startInprocTopology() before the host service Start() in %s", err, mainPath)
		}
		return false, err
	}

	insertAt := insertIndexBeforeIdentStart(mainFn, appIdent)
	if insertAt >= len(mainFn.Body.List) {
		return false, fmt.Errorf("host service %q Start() not found in main()", appIdent)
	}

	stmts := buildStartInprocTopologyStmts(mainFn, appIdent)
	mainFn.Body.List = append(mainFn.Body.List[:insertAt], append(stmts, mainFn.Body.List[insertAt:]...)...)

	if err := writeGoFile(host.fset, file, mainPath); err != nil {
		return false, err
	}
	return true, nil
}

func hostAppIdentFromServiceConstructor(mainFn *ast.FuncDecl, servicePkg, serviceName string, consts map[string]string) (string, error) {
	if mainFn == nil || mainFn.Body == nil {
		return "", fmt.Errorf("main() body is nil")
	}

	var appIdent string
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.AssignStmt)
		if !ok || len(stmt.Lhs) == 0 || len(stmt.Rhs) == 0 {
			return true
		}
		call, ok := stmt.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != servicePkg {
			return true
		}
		if !isHostServiceConstructor(sel.Sel.Name) {
			return true
		}
		matches, dynamic, err := constructorMatchesServiceName(call, serviceName, consts)
		if err != nil {
			return true
		}
		if dynamic {
			return true
		}
		if !matches {
			return true
		}
		if lhs, ok := stmt.Lhs[0].(*ast.Ident); ok {
			appIdent = lhs.Name
			return false
		}
		return true
	})
	if appIdent != "" {
		return appIdent, nil
	}
	if hasDynamicHostServiceConstructor(mainFn, servicePkg) {
		return "", ErrDynamicServiceName
	}
	return "", fmt.Errorf("host service constructor for %q not found in main()", serviceName)
}

func isHostServiceConstructor(name string) bool {
	for _, constructor := range hostServiceConstructors {
		if name == constructor {
			return true
		}
	}
	return false
}

func constructorMatchesServiceName(call *ast.CallExpr, serviceName string, consts map[string]string) (matches bool, dynamic bool, err error) {
	if len(call.Args) == 0 {
		return serviceName == "main", false, nil
	}
	name, evalErr := evalStringExpr(call.Args[0], consts)
	if evalErr != nil {
		return false, true, evalErr
	}
	return name == serviceName, false, nil
}

func hasDynamicHostServiceConstructor(mainFn *ast.FuncDecl, servicePkg string) bool {
	found := false
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.AssignStmt)
		if !ok || len(stmt.Rhs) == 0 {
			return true
		}
		call, ok := stmt.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != servicePkg {
			return true
		}
		if !isHostServiceConstructor(sel.Sel.Name) {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		if _, evalErr := evalStringExpr(call.Args[0], map[string]string{}); evalErr != nil {
			found = true
			return false
		}
		return true
	})
	return found
}

func buildStartInprocTopologyStmts(mainFn *ast.FuncDecl, appIdent string) []ast.Stmt {
	startCall := &ast.CallExpr{Fun: &ast.Ident{Name: "startInprocTopology"}}
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{&ast.Ident{Name: "err"}},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{startCall},
		},
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: "err"},
			Op: token.NEQ,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.CallExpr{
				Fun:  &ast.Ident{Name: "panic"},
				Args: []ast.Expr{&ast.Ident{Name: "err"}},
			}},
		}},
	}
	if !mainUsesPanicOnStartError(mainFn, appIdent) {
		ifStmt.Body = &ast.BlockStmt{List: []ast.Stmt{
			&ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: "err"}}},
		}}
	}
	return []ast.Stmt{ifStmt}
}

func mainUsesPanicOnStartError(mainFn *ast.FuncDecl, appIdent string) bool {
	if mainFn == nil || mainFn.Body == nil || appIdent == "" {
		return true
	}
	usesPanic := true
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil {
			return true
		}
		assign, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Start" {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != appIdent {
			return true
		}
		for _, stmt := range ifStmt.Body.List {
			if _, ok := stmt.(*ast.ExprStmt); ok {
				if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
					if panicCall, ok := exprStmt.X.(*ast.CallExpr); ok {
						if panicIdent, ok := panicCall.Fun.(*ast.Ident); ok && panicIdent.Name == "panic" {
							usesPanic = true
							return false
						}
					}
				}
			}
			if _, ok := stmt.(*ast.ReturnStmt); ok {
				usesPanic = false
				return false
			}
		}
		return true
	})
	return usesPanic
}

func findSetServiceConfigLiteral(host *hostPackageAST, targetName string) (*ast.CompositeLit, error) {
	mainFn := findMainFunc(host.files)
	if mainFn == nil || mainFn.Body == nil {
		return nil, ErrNoModuleURL
	}

	var matched *ast.CompositeLit
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "SetServiceConfig" {
			return true
		}
		if _, ok := sel.X.(*ast.Ident); !ok {
			return true
		}

		lit, err := resolveServiceConfigArg(host, call.Args[0])
		if err != nil {
			return true
		}
		name, err := structStringField(lit, "Name", host.consts)
		if err != nil {
			return true
		}
		if name != targetName {
			return true
		}
		matched = lit
		return false
	})

	if matched == nil {
		return nil, ErrNoModuleURL
	}
	return matched, nil
}

func resolveServiceConfigArg(host *hostPackageAST, expr ast.Expr) (*ast.CompositeLit, error) {
	switch value := expr.(type) {
	case *ast.CompositeLit:
		return value, nil
	case *ast.CallExpr:
		ident, ok := value.Fun.(*ast.Ident)
		if !ok {
			return nil, ErrDynamicModuleURL
		}
		fn, ok := host.funcs[ident.Name]
		if !ok {
			return nil, ErrDynamicModuleURL
		}
		if len(value.Args) > 0 {
			return nil, ErrDynamicModuleURL
		}
		return returnStructLiteral(fn)
	default:
		return nil, ErrDynamicModuleURL
	}
}

func returnStructLiteral(fn *ast.FuncDecl) (*ast.CompositeLit, error) {
	if fn == nil || fn.Body == nil {
		return nil, ErrDynamicModuleURL
	}
	var lit *ast.CompositeLit
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		composite, ok := ret.Results[0].(*ast.CompositeLit)
		if !ok {
			return nil, ErrDynamicModuleURL
		}
		lit = composite
	}
	if lit == nil {
		return nil, ErrDynamicModuleURL
	}
	return lit, nil
}

func structStringField(lit *ast.CompositeLit, fieldName string, consts map[string]string) (string, error) {
	if lit == nil {
		return "", ErrNoModuleURL
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != fieldName {
			continue
		}
		return evalStringExpr(kv.Value, consts)
	}
	return "", ErrNoModuleURL
}

func findModuleURLStringLit(host *hostPackageAST, serviceName string) (*ast.BasicLit, error) {
	configLit, err := findSetServiceConfigLiteral(host, serviceName)
	if err != nil {
		return nil, err
	}
	for _, elt := range configLit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "ModuleUrl" {
			continue
		}
		return stringValueLit(host.files, kv.Value)
	}
	return nil, ErrNoModuleURL
}

func stringValueLit(files []*ast.File, expr ast.Expr) (*ast.BasicLit, error) {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return nil, ErrDynamicModuleURL
		}
		return value, nil
	case *ast.Ident:
		return findConstStringLit(files, value.Name)
	default:
		return nil, ErrDynamicModuleURL
	}
}

func findConstStringLit(files []*ast.File, name string) (*ast.BasicLit, error) {
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range valueSpec.Names {
					if ident.Name != name {
						continue
					}
					if i >= len(valueSpec.Values) {
						return nil, ErrNoModuleURL
					}
					lit, ok := valueSpec.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return nil, ErrDynamicModuleURL
					}
					return lit, nil
				}
			}
		}
	}
	return nil, ErrNoModuleURL
}

func writeHostPackage(host *hostPackageAST) error {
	if host == nil || host.fset == nil {
		return fmt.Errorf("host package ast is nil")
	}
	for _, file := range host.files {
		filePath := host.fset.File(file.Pos()).Name()
		if filePath == "" {
			return fmt.Errorf("host source file path is empty")
		}
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, host.fset, file); err != nil {
			return fmt.Errorf("printer.Fprint(%q): %w", filePath, err)
		}
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("format.Source(%q): %w", filePath, err)
		}
		if err := os.WriteFile(filePath, formatted, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", filePath, err)
		}
	}
	return nil
}
func evalStringExpr(expr ast.Expr, consts map[string]string) (string, error) {
	if value, ok := stringExpr(expr, consts); ok {
		return value, nil
	}
	return "", ErrDynamicModuleURL
}

func stringExpr(expr ast.Expr, consts map[string]string) (string, bool) {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(value.Value)
		if err != nil {
			return "", false
		}
		return unquoted, true
	case *ast.Ident:
		if consts == nil {
			return "", false
		}
		constValue, ok := consts[value.Name]
		return constValue, ok
	default:
		return "", false
	}
}

func findMainFile(host *hostPackageAST) (*ast.File, *ast.FuncDecl) {
	for _, file := range host.files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "main" && fn.Recv == nil {
				return file, fn
			}
		}
	}
	return nil, nil
}

func serviceImportLocalName(files []*ast.File) string {
	if name, ok := importLocalName(files, "github.com/noPerfection/service"); ok {
		return name
	}
	return "service"
}

func inprocVarName(localName string) string {
	return localName + "App"
}

func inprocStarted(mainFn *ast.FuncDecl, localName, varName string) bool {
	if mainFn == nil || mainFn.Body == nil {
		return false
	}
	hasNew := false
	hasStart := false
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			if len(stmt.Rhs) != 1 {
				return true
			}
			call, ok := stmt.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "New" {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == localName {
				if len(stmt.Lhs) > 0 {
					if lhs, ok := stmt.Lhs[0].(*ast.Ident); ok && lhs.Name == varName {
						hasNew = true
					}
				}
			}
		case *ast.CallExpr:
			sel, ok := stmt.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Start" {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == varName {
				hasStart = true
			}
		}
		return true
	})
	return hasNew && hasStart
}

func hostAppIdentFromNew(mainFn *ast.FuncDecl, servicePkg string) string {
	if mainFn == nil || mainFn.Body == nil {
		return ""
	}
	var appIdent string
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.AssignStmt)
		if !ok || len(stmt.Lhs) == 0 || len(stmt.Rhs) == 0 {
			return true
		}
		call, ok := stmt.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "New" {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); !ok || ident.Name != servicePkg {
			return true
		}
		if lhs, ok := stmt.Lhs[0].(*ast.Ident); ok {
			appIdent = lhs.Name
			return false
		}
		return true
	})
	return appIdent
}

func insertIndexBeforeIdentStart(mainFn *ast.FuncDecl, ident string) int {
	if mainFn == nil || mainFn.Body == nil || ident == "" {
		return len(mainFn.Body.List)
	}
	for i, stmt := range mainFn.Body.List {
		if stmtCallsIdentStart(stmt, ident) {
			return i
		}
	}
	return len(mainFn.Body.List)
}

func stmtCallsIdentStart(stmt ast.Stmt, ident string) bool {
	found := false
	ast.Inspect(stmt, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Start" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == ident {
			found = true
			return false
		}
		return true
	})
	return found
}

func ensureImportInFile(file *ast.File, importPath string) string {
	localName := path.Base(importPath)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok || importSpec.Path == nil {
				continue
			}
			pathValue, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil || pathValue != importPath {
				continue
			}
			if importSpec.Name != nil {
				return importSpec.Name.Name
			}
			return localName
		}
		gen.Specs = append(gen.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)},
		})
		return localName
	}

	importDecl := &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)},
			},
		},
	}
	insertAt := 1
	if len(file.Decls) == 0 {
		insertAt = 0
	}
	file.Decls = append(file.Decls[:insertAt], append([]ast.Decl{importDecl}, file.Decls[insertAt:]...)...)
	return localName
}
const inprocTopologyFilename = "inproc_topology.go"

const inprocTopologySkeleton = `// This file is managed by no perfection.
package main

import (
	"github.com/noPerfection/service"
)

func startInprocTopology() error {
	inprocTopology, err := service.NewInprocExtension()
	if err != nil {
		return err
	}

	return inprocTopology.Start()
}
`

// UpdateInprocTopology adds imports, service constructors, and SetService calls
// to the host main package's inproc_topology.go for each listed inproc service.
func UpdateInprocTopology(hostModuleURL string, services []config.Service) error {
	if len(services) == 0 {
		return nil
	}
	if hostModuleURL == "" {
		return fmt.Errorf("host module url is empty")
	}

	host, err := loadHostPackage(hostModuleURL)
	if err != nil {
		return fmt.Errorf("loadHostPackage: %w", err)
	}

	file, filePath, err := loadInprocTopologyFile(host)
	if err != nil {
		return err
	}

	fn := findFuncDecl(file, "startInprocTopology")
	if fn == nil || fn.Body == nil {
		return fmt.Errorf("%q: startInprocTopology not found", filePath)
	}

	inprocVar := inprocTopologyVarName(fn)
	if inprocVar == "" {
		inprocVar = "inprocTopology"
	}

	var newStmts []ast.Stmt
	var setServiceStmts []ast.Stmt
	reservedVars := inprocServiceVarNames(fn)

	for _, svc := range services {
		if inprocTopologyHasSetService(fn, svc.Name, host) {
			continue
		}

		pkgInfo, err := package_url.New(svc.ModuleUrl)
		if err != nil {
			return fmt.Errorf("package_url.New(%q): %w", svc.ModuleUrl, err)
		}
		importPath := pkgInfo.ImportClause()
		if importPath == "" {
			return fmt.Errorf("service %q has empty import clause", svc.Name)
		}

		localName := ensureImportInFile(file, importPath)
		varName, err := nextInprocServiceVarName(localName, reservedVars)
		if err != nil {
			return fmt.Errorf("nextInprocServiceVarName(%q): %w", svc.Name, err)
		}
		reservedVars[varName] = struct{}{}

		serviceNameExpr, err := findServiceNameExpr(host, svc.Name)
		if err != nil {
			return fmt.Errorf("findServiceNameExpr(%q): %w", svc.Name, err)
		}

		newStmts = append(newStmts, buildInprocServiceNewStmts(localName, varName)...)
		setServiceStmts = append(setServiceStmts, buildInprocSetServiceStmt(inprocVar, serviceNameExpr, varName))
	}

	if len(newStmts) == 0 && len(setServiceStmts) == 0 {
		return nil
	}

	insertAt := insertIndexBeforeSetService(fn, inprocVar)
	combined := append(newStmts, setServiceStmts...)
	fn.Body.List = append(fn.Body.List[:insertAt], append(combined, fn.Body.List[insertAt:]...)...)

	return writeGoFile(host.fset, file, filePath)
}

func loadInprocTopologyFile(host *hostPackageAST) (*ast.File, string, error) {
	if host == nil || host.info == nil {
		return nil, "", fmt.Errorf("host package is nil")
	}

	hostDir := hostPackageDir(host)
	for _, file := range host.files {
		filePath := host.fset.File(file.Pos()).Name()
		if filepath.Base(filePath) == inprocTopologyFilename && filepath.Dir(filePath) == hostDir {
			return file, filePath, nil
		}
	}

	filePath := filepath.Join(hostDir, inprocTopologyFilename)
	if _, err := os.Stat(filePath); err == nil {
		file, err := parser.ParseFile(host.fset, filePath, nil, parser.ParseComments)
		if err != nil {
			return nil, "", fmt.Errorf("parser.ParseFile(%q): %w", filePath, err)
		}
		return file, filePath, nil
	}

	file, err := parser.ParseFile(host.fset, filePath, inprocTopologySkeleton, parser.ParseComments)
	if err != nil {
		return nil, "", fmt.Errorf("parser.ParseFile(%q skeleton): %w", filePath, err)
	}
	return file, filePath, nil
}

func hostPackageDir(host *hostPackageAST) string {
	if host == nil {
		return ""
	}
	for _, file := range host.files {
		filePath := host.fset.File(file.Pos()).Name()
		if filepath.Base(filePath) == "main.go" {
			return filepath.Dir(filePath)
		}
	}
	if len(host.files) > 0 {
		return filepath.Dir(host.fset.File(host.files[0].Pos()).Name())
	}
	return host.info.Dir()
}

func findFuncDecl(file *ast.File, name string) *ast.FuncDecl {
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Recv != nil {
			continue
		}
		if fn.Name.Name == name {
			return fn
		}
	}
	return nil
}
func findServiceNameExpr(host *hostPackageAST, serviceName string) (ast.Expr, error) {
	lit, err := findSetServiceConfigLiteral(host, serviceName)
	if err != nil {
		return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(serviceName)}, nil
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Name" {
			continue
		}
		return kv.Value, nil
	}
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(serviceName)}, nil
}

func inprocTopologyVarName(fn *ast.FuncDecl) string {
	if fn == nil || fn.Body == nil {
		return ""
	}
	var varName string
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.AssignStmt)
		if !ok || len(stmt.Lhs) == 0 || len(stmt.Rhs) == 0 {
			return true
		}
		call, ok := stmt.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewInprocExtension" {
			return true
		}
		if lhs, ok := stmt.Lhs[0].(*ast.Ident); ok {
			varName = lhs.Name
			return false
		}
		return true
	})
	return varName
}

func inprocTopologyHasSetService(fn *ast.FuncDecl, serviceName string, host *hostPackageAST) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call := setServiceCallFromNode(node)
		if call == nil || len(call.Args) < 2 {
			return true
		}
		name, err := evalStringExpr(call.Args[0], host.consts)
		if err == nil && name == serviceName {
			found = true
			return false
		}
		return true
	})
	return found
}

func setServiceCallFromNode(node ast.Node) *ast.CallExpr {
	switch stmt := node.(type) {
	case *ast.CallExpr:
		return setServiceCallExpr(stmt)
	case *ast.IfStmt:
		if stmt.Init != nil {
			if assign, ok := stmt.Init.(*ast.AssignStmt); ok && len(assign.Rhs) == 1 {
				if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
					return setServiceCallExpr(call)
				}
			}
		}
	}
	return nil
}

func setServiceCallExpr(call *ast.CallExpr) *ast.CallExpr {
	if call == nil {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "SetService" {
		return nil
	}
	return call
}

func inprocServiceVarNames(fn *ast.FuncDecl) map[string]struct{} {
	names := make(map[string]struct{})
	if fn == nil || fn.Body == nil {
		return names
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.AssignStmt)
		if !ok || len(stmt.Lhs) == 0 {
			return true
		}
		if ident, ok := stmt.Lhs[0].(*ast.Ident); ok {
			names[ident.Name] = struct{}{}
		}
		return true
	})
	return names
}

func nextInprocServiceVarName(localName string, reserved map[string]struct{}) (string, error) {
	if localName == "" {
		return "", fmt.Errorf("local package name is empty")
	}
	max := 0
	prefix := localName
	for name := range reserved {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if suffix == "" {
			continue
		}
		n, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%d", prefix, max+1), nil
}

func insertIndexBeforeSetService(fn *ast.FuncDecl, inprocVar string) int {
	if fn == nil || fn.Body == nil {
		return 0
	}
	for i, stmt := range fn.Body.List {
		if stmtHasSetServiceOn(stmt, inprocVar) {
			return i
		}
	}
	return insertIndexBeforeInprocStart(fn, inprocVar)
}

func insertIndexBeforeInprocStart(fn *ast.FuncDecl, inprocVar string) int {
	if fn == nil || fn.Body == nil {
		return 0
	}
	for i, stmt := range fn.Body.List {
		if stmtCallsIdentStart(stmt, inprocVar) {
			return i
		}
	}
	return len(fn.Body.List)
}

func stmtHasSetServiceOn(stmt ast.Stmt, inprocVar string) bool {
	found := false
	ast.Inspect(stmt, func(node ast.Node) bool {
		call := setServiceCallFromNode(node)
		if call == nil {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == inprocVar {
			found = true
			return false
		}
		return true
	})
	return found
}

func buildInprocServiceNewStmts(localName, varName string) []ast.Stmt {
	newCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: localName},
			Sel: &ast.Ident{Name: "New"},
		},
	}
	return []ast.Stmt{
		&ast.AssignStmt{
			Lhs: []ast.Expr{&ast.Ident{Name: varName}, &ast.Ident{Name: "err"}},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{newCall},
		},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "err"},
				Op: token.NEQ,
				Y:  &ast.Ident{Name: "nil"},
			},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: "err"}}},
			}},
		},
	}
}

func buildInprocSetServiceStmt(inprocVar string, serviceNameExpr ast.Expr, varName string) ast.Stmt {
	setCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: inprocVar},
			Sel: &ast.Ident{Name: "SetService"},
		},
		Args: []ast.Expr{serviceNameExpr, &ast.Ident{Name: varName}},
	}
	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{&ast.Ident{Name: "err"}},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{setCall},
		},
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: "err"},
			Op: token.NEQ,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: "err"}}},
		}},
	}
}

func writeGoFile(fset *token.FileSet, file *ast.File, filePath string) error {
	if fset == nil || file == nil {
		return fmt.Errorf("file ast is nil")
	}
	if filePath == "" {
		return fmt.Errorf("file path is empty")
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		return fmt.Errorf("printer.Fprint(%q): %w", filePath, err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format.Source(%q): %w", filePath, err)
	}
	if err := os.WriteFile(filePath, formatted, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", filePath, err)
	}
	return nil
}
