//go:build windows
// +build windows

package dev

import (
	"path/filepath"
)

func (dep *Dep) BinPath() string {
	return filepath.Join(dep.context.config.Bin, dep.url+"/bin.exe")
}
