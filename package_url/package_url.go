package package_url

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

type PackageInfo struct {
	// File that calls this package, traverses in the stack trace to until it doesn't find main.main
	mainFile string
	// Module path, in golang modules are called packages, but we use purl convention
	modulePath string
	// Package path, in golang packages are called modules, but we use purl convention
	packagePath string
}

const trimpathFlaggedError = "you run it with trimpath flag. To show full path please dont use it."

// GetPackageInfo returns the package info for the main package.
// If you run it with trimpath flag, it will return an error.
func GetPackageInfo() (*PackageInfo, error) {
	mainFile, err := mainFile()
	if err != nil {
		return nil, err
	}

	mainPackage, goModPackage, err := fileToModuleAndPackage(mainFile)
	if err != nil {
		return nil, err
	}

	mainFileAbsolute := filepath.IsAbs(mainFile)
	if !mainFileAbsolute {
		return nil, fmt.Errorf(trimpathFlaggedError)
	}
	return &PackageInfo{
		mainFile:    mainFile,
		modulePath:  mainPackage,
		packagePath: goModPackage,
	}, nil
}

func (info *PackageInfo) Print() {
	fmt.Println("main.go:         ", info.mainFile)
	fmt.Println("main package:    ", info.modulePath)
	fmt.Println("go.mod package:  ", info.packagePath)
}

func FillDefaultModuleURL() (string, error) {
	moduleURL, err := GetPackageInfo()
	if err != nil {
		return "", err
	}
	return moduleURL.modulePath, nil
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

// returns first the main package, and then the go.mod package
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

func mainPackageFromGomod(mainFile string) (string, string, error) {
	dir := filepath.Dir(mainFile)
	for {
		goModPath := filepath.Join(dir, "go.mod")
		goMod, err := os.ReadFile(goModPath)
		if err == nil {
			moduleName, err := modulePath(goMod)
			if err != nil {
				return "", "", fmt.Errorf("%s: %w", goModPath, err)
			}

			rel, err := filepath.Rel(dir, filepath.Dir(mainFile))
			if err != nil {
				return "", "", err
			}
			if rel == "." {
				return moduleName, moduleName, nil
			}
			return moduleName + "/" + filepath.ToSlash(rel), moduleName, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", fmt.Errorf("go.mod not found for %s", mainFile)
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
