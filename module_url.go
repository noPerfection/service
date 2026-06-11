package service

import (
	"errors"
	"runtime/debug"

	"github.com/noPerfection/topology/config"
)

const moduleURLBuildInfoError = "can't find the module url. Please build it as a normal module or update Go to a latest version"

var readBuildInfo = debug.ReadBuildInfo

func mainModuleURL() (string, error) {
	info, ok := readBuildInfo()
	if !ok || info == nil || info.Main.Path == "" {
		return "", errors.New(moduleURLBuildInfoError)
	}
	return info.Main.Path, nil
}

func fillDefaultModuleURL(service *config.Service) error {
	if service == nil || service.ModuleUrl != "" {
		return nil
	}
	moduleURL, err := mainModuleURL()
	if err != nil {
		return err
	}
	service.ModuleUrl = moduleURL
	return nil
}
