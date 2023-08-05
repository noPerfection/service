//go:build windows
// +build windows

package dev

import (
	"path/filepath"
)

func (c *Context) BinPath(url string) string {
	return filepath.Join(c.Bin, url+"/bin.exe")
}
