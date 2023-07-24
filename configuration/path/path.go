package path

import (
	"os"
	"path"
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
	if path.IsAbs(mainPath) {
		return mainPath
	}

	return path.Join(execPath, mainPath)
}
