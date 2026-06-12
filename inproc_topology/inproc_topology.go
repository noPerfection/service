package inproc_topology

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

type InprocTopology struct{}

func (topology *InprocTopology) Start() error {
	mainFile, err := mainFile()
	if err != nil {
		return err
	}

	mainPackage, err := mainPackage(mainFile)
	if err != nil {
		return err
	}

	mainFileAbsolute := filepath.IsAbs(mainFile)
	fmt.Println("main.go:", mainFile)
	fmt.Println("main.go absolute:", mainFileAbsolute)
	if !mainFileAbsolute {
		fmt.Println("you run it with trimpath flag. To show full path please dont use it.")
	}
	fmt.Println("main package:", mainPackage)
	return nil
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

func mainPackage(mainFile string) (string, error) {
	info, ok := debug.ReadBuildInfo()
	if ok && info != nil && info.Path != "" && info.Path != "command-line-arguments" {
		return info.Path, nil
	}
	if ok && info != nil && info.Path == "command-line-arguments" && mainFile == "./main.go" {
		return "", fmt.Errorf("build using package path go build ./cmd/service, not file path go build ./cmd/service/main.go")
	}

	if mainPackage, err := mainPackageFromGomod(mainFile); err == nil {
		return mainPackage, nil
	}
	if ok && info != nil && info.Path != "" {
		return info.Path, nil
	}

	return "", fmt.Errorf("main package not found")
}

func mainPackageFromGomod(mainFile string) (string, error) {
	dir := filepath.Dir(mainFile)
	for {
		goModPath := filepath.Join(dir, "go.mod")
		goMod, err := os.ReadFile(goModPath)
		if err == nil {
			moduleName, err := modulePath(goMod)
			if err != nil {
				return "", fmt.Errorf("%s: %w", goModPath, err)
			}

			rel, err := filepath.Rel(dir, filepath.Dir(mainFile))
			if err != nil {
				return "", err
			}
			if rel == "." {
				return moduleName, nil
			}
			return moduleName + "/" + filepath.ToSlash(rel), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found for %s", mainFile)
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
