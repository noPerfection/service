//go:build windows
// +build windows

package dev

import (
	"github.com/ahmetson/service-lib/configuration"
	"path"
)

func BinPath(context *configuration.Context, url string) string {
	return path.Join(context.Bin, url+"/bin.exe")
}
