package package_url

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/ahmetson/mushroom"
	"golang.org/x/tools/go/packages"
)

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
}

const trimpathFlaggedError = "you run it with trimpath flag. To show full path please dont use it."

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

func (info *PackageInfo) String() string {
	return info.mushroomHypha.String()
}

func FillDefaultModuleURL() (string, error) {
	moduleURL, err := GetPackageInfo()
	if err != nil {
		return "", err
	}
	return moduleURL.module, nil
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
//	- {go.mod module path} — must match the module directive in go.mod at root.
//	- #{package fragment} — path under the module (for example /cmd/service). Empty when the package is the module root.
//
// Additional props:
//
//   - root — filesystem directory that contains go.mod. When omitted, the current working directory is used
//     and written back onto the parsed hypha.
//   - main — when set to true or false, the package is loaded with go/packages and the package clause must match
//     (main for true, any other name for false). For example, main=true on a library package fails.
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

	root, err := resolveNewRoot(&hypha)
	if err != nil {
		return nil, err
	}

	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil, fmt.Errorf("go.mod not found at %q", root)
	}

	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}
	goModModule, err := modulePath(goMod)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", goModPath, err)
	}
	if hypha.PackageID != goModModule {
		return nil, fmt.Errorf("mushroom package %q does not match go.mod module %q", hypha.PackageID, goModModule)
	}

	importPath, relFragment, err := importPathFromNewHypha(hypha)
	if err != nil {
		return nil, err
	}

	loadPattern := "."
	moduleDir := root
	if relFragment != "" {
		loadPattern = "./" + filepath.ToSlash(relFragment)
		moduleDir = filepath.Join(root, filepath.FromSlash(relFragment))
	}

	pkgs, err := packages.Load(&packages.Config{
		Dir:  root,
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

	return &PackageInfo{
		moduleDir:     pkg.Dir,
		mainModule:    isMain,
		module:        importPath,
		pkg:           goModModule,
		pkgDir:        root,
		mushroomHypha: hypha,
	}, nil
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

func importPathFromNewHypha(hypha mushroom.Hypha) (importPath string, relFragment string, err error) {
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
