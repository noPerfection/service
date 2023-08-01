//go:build !windows
// +build !windows

package dev

import (
	"path"
)

func (dep *Dep) BinPath() string {
	return path.Join(dep.context.config.Bin, dep.url+"/bin")
}
