package package_url

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/ahmetson/mushroom"
	ospath "github.com/noPerfection/os/path"
	"golang.org/x/tools/go/packages"
)

const thirdPartyProp = "thirdparty"

// ErrThirdPartyNotEditable is returned when a mushroom link points at a go.mod
// dependency that has no local replace directive, so its source cannot be edited.
var ErrThirdPartyNotEditable = errors.New("third-party package is not editable")

type PackageInfo struct {
	// File that calls this package, traverses in the stack trace to until it doesn't find main.main
	moduleDir  string
	mainModule bool
	// Module path, in golang modules are called packages, but we use purl convention
	module string
	// Package directory, the root with the go.mod file
	pkgDir string
	// Package path, in golang packages are called modules, but we use purl convention
	pkg           string
	mushroomHypha mushroom.Hypha
	// source files
	sourceFiles []string
	thirdParty  bool
}

const trimpathFlaggedError = "you run it with trimpath flag. To show full path please dont use it."

// ServiceNameToPackageName normalizes a service name into a Go package name fragment.
// It trims surrounding space, collapses internal whitespace, and replaces hyphens with underscores.
func ServiceNameToPackageName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "-", "_")
	return strings.Join(strings.Fields(name), "_")
}

// GetPackageInfo returns the package info for the current app
// If you run it with trimpath flag, it will return an error.
func GetPackageInfo() (*PackageInfo, error) {
	mainFile, err := mainFile()
	if err != nil {
		return nil, err
	}

	mainFileAbsolute := filepath.IsAbs(mainFile)
	if !mainFileAbsolute {
		return nil, fmt.Errorf(trimpathFlaggedError)
	}

	mainModule, mainPackage, err := fileToModuleAndPackage(mainFile)
	if err != nil {
		return nil, err
	}
	goModDir, _, err := getGoMod(mainFile)
	if err != nil {
		return nil, err
	}

	mushroomURL := fmt.Sprintf("pkg:golang/%s#%s?root=%s&main=true", mainPackage, strings.ReplaceAll(mainModule, mainPackage, ""), goModDir)
	soil := mushroom.Soil{}
	hypha, err := soil.Hypha(mushroomURL)
	if err != nil {
		return nil, err
	}
	return &PackageInfo{
		moduleDir:     filepath.Dir(mainFile),
		module:        mainModule,
		mainModule:    true,
		pkg:           mainPackage,
		pkgDir:        goModDir,
		mushroomHypha: hypha,
		sourceFiles:   []string{mainFile},
	}, nil
}

func (info *PackageInfo) Print() {
	fmt.Println("Main module dir:        ", info.moduleDir)
	fmt.Println("Main module:            ", info.module)
	fmt.Println("go.mod dir:             ", info.pkgDir)
	fmt.Println("Main package (go.mod):  ", info.pkg)
}

func (info *PackageInfo) MushroomLink() mushroom.Hypha {
	return info.mushroomHypha.AsLink()
}

func (info *PackageInfo) Dir() string {
	return info.pkgDir
}

// IsModuleExist checks whether module exists under the same package root as info.
// It rewrites the mushroom link module fragment, drops main=true, and resolves it with New.
func (info *PackageInfo) IsModuleExist(module string) (bool, error) {
	if info == nil {
		return false, fmt.Errorf("package info is nil")
	}

	link := info.MushroomLink()
	link.ModuleID = module
	if link.AdditionalProps != nil {
		delete(link.AdditionalProps, "main")
	}

	if _, err := New(link.String()); err != nil {
		return false, nil
	}

	return true, nil
}

func (info *PackageInfo) NewModule(module string, sourceFile string) *PackageInfo {
	link := info.MushroomLink()
	link.ModuleID = module
	if link.AdditionalProps != nil {
		delete(link.AdditionalProps, "main")
	}

	pkgInfo := &PackageInfo{
		moduleDir:     module,
		module:        info.module,
		mainModule:    false,
		pkg:           info.pkg,
		pkgDir:        info.pkgDir,
		mushroomHypha: link,
		sourceFiles:   []string{sourceFile},
	}
	return pkgInfo
}

func (info *PackageInfo) String() string {
	return info.mushroomHypha.String()
}

// Returns true if the package is the main module
func (info *PackageInfo) IsMain() bool {
	return info.mainModule
}

// IsThirdParty reports whether the mushroom link resolved through a go.mod require
// rather than the workspace module directive.
func (info *PackageInfo) IsThirdParty() bool {
	return info != nil && info.thirdParty
}

// IsEditable reports whether source files for this package are available locally.
func (info *PackageInfo) IsEditable() bool {
	return info != nil && len(info.sourceFiles) > 0
}

// EnsureEditable returns an error when the package cannot be edited on disk.
func (info *PackageInfo) EnsureEditable() error {
	if info == nil {
		return fmt.Errorf("package info is nil")
	}
	if len(info.sourceFiles) == 0 {
		if info.thirdParty {
			return ErrThirdPartyNotEditable
		}
		return fmt.Errorf("package %q has no source files on disk", info.ImportClause())
	}
	return nil
}

func (info *PackageInfo) SourceFiles() []string {
	return info.sourceFiles
}

// ImportClause returns the Go import path for this package module,
// using the same resolution as package_url.New.
func (info *PackageInfo) ImportClause() string {
	if info == nil {
		return ""
	}
	importPath, _, _ := importClause(info.MushroomLink())
	return importPath
}

// PackageName returns the Go package name from this module's mushroom ModuleID.
func (info *PackageInfo) PackageName() string {
	if info == nil {
		return ""
	}
	moduleID := info.MushroomLink().ModuleID
	if moduleID == "" {
		return ""
	}
	parts := strings.Split(moduleID, "/")
	return parts[len(parts)-1]
}

func FillDefaultModuleURL() (string, error) {
	moduleURL, err := GetPackageInfo()
	if err != nil {
		return "", err
	}
	return moduleURL.String(), nil
}

func mainFile() (string, error) {
	var pcs [64]uintptr
	n := runtime.Callers(0, pcs[:])

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, "main.") {
			return frame.File, nil
		}
		if !more {
			break
		}
	}

	if mainFile, ok := mainFileFromAllGoroutines(); ok {
		return mainFile, nil
	}

	return "", fmt.Errorf("main package frame not found")
}

func mainFileFromAllGoroutines() (string, bool) {
	bufferSize := 64 * 1024
	for {
		buffer := make([]byte, bufferSize)
		n := runtime.Stack(buffer, true)
		if n < len(buffer) {
			return mainFileFromStack(string(buffer[:n]))
		}
		bufferSize *= 2
	}
}

func mainFileFromStack(stack string) (string, bool) {
	lines := strings.Split(stack, "\n")
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "main.") || index+1 >= len(lines) {
			continue
		}

		fileLine := strings.Fields(strings.TrimSpace(lines[index+1]))
		if len(fileLine) == 0 {
			continue
		}

		file := fileLine[0]
		lineNumber := strings.LastIndex(file, ":")
		if lineNumber == -1 {
			continue
		}
		return file[:lineNumber], true
	}
	return "", false
}

// returns first the main package, and then the go.mod package.
// Uses mushroom url convention, not golang module path convention.
// So modules are named package, and packages are named modules.
func fileToModuleAndPackage(mainFile string) (string, string, error) {
	info, ok := debug.ReadBuildInfo()
	if ok && info != nil && info.Path != "" && info.Path != "command-line-arguments" {
		return info.Path, info.Main.Path, nil
	}
	if ok && info != nil && info.Path == "command-line-arguments" && mainFile == "./main.go" {
		return "", "", fmt.Errorf("build using package path go build ./cmd/service, not file path go build ./cmd/service/main.go")
	}

	if mainPackage, goModPackage, err := mainPackageFromGomod(mainFile); err == nil {
		return mainPackage, goModPackage, nil
	}

	return "", "", fmt.Errorf("main package not found")
}

// Returns the directory and file of go.mod for the given file
func getGoMod(mainFile string) (string, []byte, error) {
	dir := filepath.Dir(mainFile)
	for {
		goModPath := filepath.Join(dir, "go.mod")
		goMod, err := os.ReadFile(goModPath)
		if err == nil {
			return dir, goMod, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", nil, fmt.Errorf("go.mod not found for %s", mainFile)
}

// Returns the main package and the go.mod module by traversing from the main file to the go.mod directory.
func mainPackageFromGomod(mainFile string) (string, string, error) {
	goModDir, goMod, err := getGoMod(mainFile)
	if err != nil {
		return "", "", err
	}
	moduleName, err := modulePath(goMod)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", goModDir+"/go.mod", err)
	}

	rel, err := filepath.Rel(goModDir, filepath.Dir(mainFile))
	if err != nil {
		return "", "", err
	}
	if rel == "." {
		return moduleName, moduleName, nil
	}
	return moduleName + "/" + filepath.ToSlash(rel), moduleName, nil
}

func modulePath(goMod []byte) (string, error) {
	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found")
}

// New parses and verifies a golang mushroom link and resolves package metadata on disk.
//
// Only pkg:golang links are accepted. Dereference URLs (prefixed with *) fail.
// Symbolic names (for example "hello-world") and non-golang types (for example pkg:json/...) fail too.
// URLs with a resource query (var=, func=, or obj=) fail.
//
// URL shape:
//
//	pkg:golang/{go.mod module path}#{package fragment}?root={filesystem path}&main={true|false}
//
//	- pkg:golang/... — package type must be golang.
//	- {go.mod module path} — workspace module path, or a module listed in go.mod require (see thirdparty below).
//	- #{package fragment} — path under the module (for example /cmd/service). Empty when the package is the module root.
//
// Additional props:
//
//   - root — filesystem directory that contains go.mod. When omitted, the current working directory is used
//     and written back onto the parsed hypha.
//   - main — when set to true or false, the package is loaded with go/packages and the package clause must match
//     (main for true, any other name for false). For example, main=true on a library package fails.
//   - thirdparty — set to true when the mushroom package path is resolved from a go.mod require rather than
//     the workspace module. Omitted for the local module.
//
// When the mushroom package path does not match the workspace go.mod module, New looks up the path in
// go.mod require directives. If a replace directive points at a local directory, source files are loaded
// from that directory. Without a replace, New still succeeds but returns no source files.
//
// Examples:
//
//	// Main package in examples/009-inproc-services/cmd/service
//	New("pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#/cmd/service?root=/home/user/noPerfection/service/examples/009-inproc-services&main=true")
//
//	// Same module, root taken from the current working directory when root is omitted
//	New("pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#/cmd/service?main=true")
//
//	// Library package under the same module
//	New("pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#/internal/foo?root=/home/user/noPerfection/service/examples/009-inproc-services&main=false")
//
//	// Third-party module resolved through go.mod require + replace
//	New("pkg:golang/github.com/noPerfection/service?root=/home/user/noPerfection/service/examples/009-inproc-services")
//
// Wrong URLs (all return an error):
//
//	New("hello-world")                                                                         // symbolic, not a link
//	New("*pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#/cmd/service") // dereference
//	New("pkg:json/github.com/foo/config.json#main")                                            // wrong type
//	New("pkg:golang/github.com/foo/bar?var=services")                                          // resource query, not additional props
//	New("pkg:golang/github.com/foo/bar/cmd?main=true")                                         // main=true but package clause is not main
func New(mushroomURL string) (*PackageInfo, error) {
	soil := mushroom.Soil{}
	hypha, err := soil.Hypha(mushroomURL)
	if err != nil {
		return nil, fmt.Errorf("parse mushroom url: %w", err)
	}
	if err := validateNewMushroomHypha(hypha); err != nil {
		return nil, err
	}

	workspaceRoot, err := resolveNewRoot(&hypha)
	if err != nil {
		return nil, err
	}

	goModPath := filepath.Join(workspaceRoot, "go.mod")
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}
	workspaceModule, err := modulePath(goMod)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", goModPath, err)
	}

	importPath, relFragment, err := importClause(hypha)
	if err != nil {
		return nil, err
	}

	loadRoot := workspaceRoot
	thirdParty := false
	resolvedModule := workspaceModule

	if hypha.PackageID != workspaceModule {
		requiredModule, ok := findRequiredModule(hypha.PackageID, parseRequires(goMod))
		if !ok {
			return nil, fmt.Errorf("mushroom package %q is not the workspace module %q and is not listed in go.mod requirements", hypha.PackageID, workspaceModule)
		}

		thirdParty = true
		resolvedModule = requiredModule
		if hypha.AdditionalProps == nil {
			hypha.AdditionalProps = map[string]string{}
		}
		hypha.AdditionalProps[thirdPartyProp] = "true"

		replaceRoot, hasReplace := parseReplaces(goMod, workspaceRoot)[requiredModule]
		if !hasReplace {
			return newThirdPartyInfo(hypha, workspaceRoot, resolvedModule, importPath), nil
		}

		replaceGoMod, err := os.ReadFile(filepath.Join(replaceRoot, "go.mod"))
		if err != nil {
			return nil, fmt.Errorf("read replaced module go.mod at %q: %w", replaceRoot, err)
		}
		replaceModule, err := modulePath(replaceGoMod)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(replaceRoot, "go.mod"), err)
		}
		if replaceModule != requiredModule {
			return nil, fmt.Errorf("replace %q points at %q with module %q, want %q", requiredModule, replaceRoot, replaceModule, requiredModule)
		}
		loadRoot = replaceRoot
	}

	loadPattern := "."
	moduleDir := loadRoot
	if relFragment != "" {
		loadPattern = "./" + filepath.ToSlash(relFragment)
		moduleDir = filepath.Join(loadRoot, filepath.FromSlash(relFragment))
	}

	pkgs, err := packages.Load(&packages.Config{
		Dir:  loadRoot,
		Mode: packages.NeedName | packages.NeedModule | packages.NeedFiles,
		Env:  os.Environ(),
	}, loadPattern)
	if err != nil {
		return nil, fmt.Errorf("packages.Load(%q): %w", loadPattern, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("packages.Load(%q): no packages returned", loadPattern)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("packages.Load(%q): %s", loadPattern, pkg.Errors[0])
	}

	isMain := pkg.Name == "main"
	if mainProp, ok := hypha.AdditionalProps["main"]; ok {
		wantMain := mainProp == "true"
		if wantMain != isMain {
			return nil, fmt.Errorf("main=%q but package %q has name %q", mainProp, importPath, pkg.Name)
		}
	}

	if pkg.PkgPath != importPath {
		return nil, fmt.Errorf("resolved import path %q does not match mushroom import path %q", pkg.PkgPath, importPath)
	}
	if pkg.Dir != "" && pkg.Dir != moduleDir {
		return nil, fmt.Errorf("package directory %q does not match mushroom package directory %q", pkg.Dir, moduleDir)
	}
	if isMain {
		hypha.AdditionalProps["main"] = "true"
	} else {
		delete(hypha.AdditionalProps, "main")
	}

	ignored := make(map[string]struct{}, len(pkg.IgnoredFiles))
	for _, f := range pkg.IgnoredFiles {
		ignored[f] = struct{}{}
	}

	sourceFiles := slices.DeleteFunc(append(pkg.GoFiles, pkg.EmbedFiles...), func(file string) bool {
		_, skip := ignored[file]
		return skip
	})

	return &PackageInfo{
		moduleDir:     pkg.Dir,
		mainModule:    isMain,
		module:        importPath,
		pkg:           resolvedModule,
		pkgDir:        loadRoot,
		mushroomHypha: hypha,
		sourceFiles:   sourceFiles,
		thirdParty:    thirdParty,
	}, nil
}

func newThirdPartyInfo(hypha mushroom.Hypha, workspaceRoot, requiredModule, importPath string) *PackageInfo {
	if hypha.AdditionalProps == nil {
		hypha.AdditionalProps = map[string]string{}
	}
	hypha.AdditionalProps[thirdPartyProp] = "true"
	delete(hypha.AdditionalProps, "main")

	return &PackageInfo{
		moduleDir:     "",
		mainModule:    false,
		module:        importPath,
		pkg:           requiredModule,
		pkgDir:        workspaceRoot,
		mushroomHypha: hypha,
		sourceFiles:   nil,
		thirdParty:    true,
	}
}

func findRequiredModule(packageID string, requires []string) (string, bool) {
	for _, required := range requires {
		if packageID == required || strings.HasPrefix(packageID, required+"/") {
			return required, true
		}
	}
	return "", false
}

func parseRequires(goMod []byte) []string {
	var requires []string
	inBlock := false

	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "require ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "require "))
			if rest == "(" {
				inBlock = true
				continue
			}
			if module := requireModuleFromLine(rest); module != "" {
				requires = append(requires, module)
			}
			continue
		}

		if inBlock {
			if line == ")" {
				inBlock = false
				continue
			}
			if module := requireModuleFromLine(line); module != "" {
				requires = append(requires, module)
			}
		}
	}

	return requires
}

func requireModuleFromLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"`)
}

func parseReplaces(goMod []byte, goModDir string) map[string]string {
	replaces := make(map[string]string)
	inBlock := false

	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "replace ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "replace "))
			if rest == "(" {
				inBlock = true
				continue
			}
			addReplaceLine(rest, goModDir, replaces)
			continue
		}

		if inBlock {
			if line == ")" {
				inBlock = false
				continue
			}
			addReplaceLine(line, goModDir, replaces)
		}
	}

	return replaces
}

func addReplaceLine(line, goModDir string, replaces map[string]string) {
	parts := strings.SplitN(line, "=>", 2)
	if len(parts) != 2 {
		return
	}

	oldModule := strings.Trim(strings.TrimSpace(parts[0]), `"`)
	newPath := strings.Trim(strings.TrimSpace(parts[1]), `"`)
	if oldModule == "" || newPath == "" {
		return
	}
	if !filepath.IsAbs(newPath) {
		newPath = filepath.Join(goModDir, newPath)
	}
	absPath, err := filepath.Abs(newPath)
	if err != nil {
		return
	}
	replaces[oldModule] = absPath
}

func validateNewMushroomHypha(hypha mushroom.Hypha) error {
	if !hypha.URL {
		return fmt.Errorf("mushroom url %q is symbolic, want pkg:golang link", hypha.Path)
	}
	if hypha.Dereference {
		return fmt.Errorf("mushroom url %q is a dereference, want a link", hypha.String())
	}
	if hypha.Type != "golang" {
		return fmt.Errorf("mushroom url type %q is not golang", hypha.Type)
	}
	if hypha.PackageID == "" || hypha.PackageID == "$" {
		return fmt.Errorf("mushroom url is missing golang package path")
	}
	if hypha.ResourceKind != "" {
		return fmt.Errorf("mushroom url %q has resource %s, want additional props only", hypha.String(), hypha.ResourceKind)
	}
	return nil
}

// Adds the root directory parameter to the package info, if it is not set.
// Returns the directory where this project resides in either from additional root parameter or os.Getwd().
func resolveNewRoot(hypha *mushroom.Hypha) (string, error) {
	root := ""
	if hypha.AdditionalProps != nil {
		root = hypha.AdditionalProps["root"]
	}
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve root from working directory: %w", err)
		}
		root = wd
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	if hypha.AdditionalProps == nil {
		hypha.AdditionalProps = map[string]string{}
	}
	hypha.AdditionalProps["root"] = absRoot
	return absRoot, nil
}

// importClause maps a golang mushroom link to a Go import path and a
// directory fragment relative to the go.mod root (the root additional prop).
//
// importPath is the full path used in Go import statements and returned by
// PackageInfo.ImportClause. relFragment is the subdirectory under the module root
// where the package lives; New uses it to build packages.Load("./…") and moduleDir.
// When the module fragment is empty, the package is the module root and relFragment
// is "" (load pattern ".").
//
// Examples (PackageID = github.com/noPerfection/service/examples/009-inproc-services):
//
//	pkg:golang/#/cmd/service?main=true  → importPath {PackageID}/cmd/service,  relFragment cmd/service
//	pkg:golang/#services/foo            → importPath {PackageID}/services/foo, relFragment services/foo
//	pkg:golang/#(no fragment)            → importPath {PackageID}/009-inproc-services, relFragment ""
func importClause(hypha mushroom.Hypha) (importPath string, relFragment string, err error) {
	fragment := hypha.ModuleID
	if fragment == "$" {
		fragment = ""
	}

	if fragment == "" {
		return hypha.PackageID, "", nil
	}

	relFragment = strings.Trim(strings.TrimPrefix(fragment, "/"), "/")
	if relFragment == "" {
		return hypha.PackageID, "", nil
	}

	return hypha.PackageID + "/" + relFragment, relFragment, nil
}

// IsFileExist parses moduleURL into package metadata and checks whether filename
// exists in the package directory (moduleDir).
func IsFileExist(moduleURL, filename string) (bool, error) {
	info, err := New(moduleURL)
	if err != nil {
		return false, err
	}
	if err := info.EnsureEditable(); err != nil {
		return false, err
	}
	if info.moduleDir == "" {
		return false, fmt.Errorf("package %q has no package directory on disk", info.ImportClause())
	}

	path := filepath.Join(info.moduleDir, filename)
	exist, err := ospath.FileExist(path)
	if err != nil {
		return false, err
	}
	if !exist {
		return false, fmt.Errorf("%s doesn't exist", path)
	}

	return true, nil
}
