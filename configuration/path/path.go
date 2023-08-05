package path

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetExecPath returns the current path of the executable
func GetExecPath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	exPath := filepath.Dir(ex)
	return exPath, nil
}

// GetPath returns the path related to the execPath.
// If the path itself is absolute, then it's returned directly
func GetPath(execPath string, mainPath string) string {
	if filepath.IsAbs(mainPath) {
		return mainPath
	}

	return filepath.Join(execPath, mainPath)
}

// SplitServicePath returns the directory, file name without extension part.
//
// The function doesn't validate the path.
// Therefore, call this function after validateServicePath()
func SplitServicePath(servicePath string) (string, string) {
	dir, fileName := filepath.Split(servicePath)

	if len(dir) == 0 {
		dir = "."
	}

	fileName = fileName[0 : len(fileName)-4]

	return dir, fileName
}

// FileExists returns true if the file exists. if the path is a directory, it will return false.
func FileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, fmt.Errorf("os.Stat('%s'): %w", path, err)
		}
	}

	if info.IsDir() {
		return false, fmt.Errorf("path('%s') is directory", path)
	}

	return true, nil
}
