//go:build windows
// +build windows

package dev

import (
	"github.com/ahmetson/service-lib/configuration"
	"path/filepath"
)

func BinPath(context *configuration.Context, url string) string {
	return filepath.Join(context.Bin, url+"/bin.exe")
}
