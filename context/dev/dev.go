// Package dev handles the dependencies in the development environment
// which means it's in the current machine.
//
// The dependencies are including the extensions and proxies.
//
// How it works?
//
// The context is set up. It checks the folder. and if they are not existing, it will create them.
// >> dev.Prepare(context)
//
// then lets work on the extension.
// user is passing an extension url.
// the service is checking whether it exists in the data or not.
// if the service exists, it gets the yaml. and returns the configuration.
//
// if the service doesn't exist, it checks whether the service exists in the bin.
// if it exists, then it runs it with --build-configuration.
//
// then, if the service doesn't exist in the bin, it checks the source.
// if the source exists, then it will call `go build`.
// then call bin file with the generated files.
//
// lastly, if source doesn't exist, it will download the files from the repository using go-git.
// then we build the binary.
// we generate configuration.
//
// lastly, the service.Prepare() will make sure that all binaries exist.
// if not then it will create them.
//
// -----------------------------------------------
// running the application will do the following.
// it checks is the port of proxies are in use. if it's not, then it will call a run.
//
// then it will call itself.
//
// the service will have a command to "shutdown" contexts. as well as "rebuild"
package dev

import (
	"errors"
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/log"
	"os"
	"path"
)

func preparePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0777); err != nil {
				return fmt.Errorf("failed to create a directory at '%s' path: %w", path, err)
			}
			return nil
		} else {
			return fmt.Errorf("failed to read '%s': %w", path, err)
		}
	}

	if !info.IsDir() {
		return fmt.Errorf("the path '%s' is not a directory", path)
	}

	return nil
}

// Prepare the context by validating the paths. If they don't exist, then it will be created.
func Prepare(context *configuration.Context) error {
	if err := preparePath(context.Src); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if err := preparePath(context.Bin); err != nil {
		return fmt.Errorf("invalid bin path: %w", err)
	}
	if err := preparePath(context.Data); err != nil {
		return fmt.Errorf("invalid data path: %w", err)
	}

	return nil
}

// ConfigurationPath returns configuration url in the context's data
func ConfigurationPath(context *configuration.Context, url string) string {
	return path.Join(context.Data, url+"/service.yml")
}

func ConfigurationExist(context *configuration.Context, url string) (bool, error) {
	dataPath := ConfigurationPath(context, url)
	println("the data path is", dataPath)
	_, err := os.Stat(dataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to read the stat of %s: %w", dataPath, err)
		}
	}
	return true, nil
}

func prepareConfigurationPath(context *configuration.Context, url string) error {
	dir := path.Dir(ConfigurationPath(context, url))
	return preparePath(dir)
}

func prepareBinPath(context *configuration.Context, url string) error {
	dir := path.Dir(BinPath(context, url))
	return preparePath(dir)
}

func prepareSrcPath(context *configuration.Context, url string) error {
	dir := path.Dir(SrcPath(context, url))
	return preparePath(dir)
}

func BinExist(context *configuration.Context, url string) (bool, error) {
	dataPath := BinPath(context, url)
	println("the bin path is", dataPath)
	_, err := os.Stat(dataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to read the stat of %s: %w", dataPath, err)
		}
	}
	return true, nil
}

func SrcPath(context *configuration.Context, url string) string {
	return path.Join(context.Data, url)
}

func SrcExist(context *configuration.Context, url string) (bool, error) {
	dataPath := SrcPath(context, url)
	println("the source path is", dataPath)
	_, err := os.Stat(dataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to read the stat of %s: %w", dataPath, err)
		}
	}
	return true, nil
}

// PrepareProxyConfiguration returns the proxy parameters and the configuration.Proxy
func PrepareProxyConfiguration(context *configuration.Context, url string, logger *log.Logger) error {
	exist, err := ConfigurationExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check existence of %s in %s context: %w", url, context.Type, err)
	}

	if exist {
		logger.Info("required proxy exists return it by calling readProxy")
	} else {
		// first need to prepare the configuration
		err := prepareConfigurationPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareConfigurationPath: %w", err)
		}
		logger.Warn("required proxy doesn't exist continue")
	}

	// check is binary exist
	binExist, err := BinExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s context: %w", url, context.Type, err)
	}

	if binExist {
		logger.Info("binary exists, we need to call it with --generate-config by calling generateConfig() and readProxy()")
	} else {
		// first need to prepare the directory
		err := prepareBinPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareBinPath: %w", err)
		}
		logger.Warn("binary doesn't exist, continue")
	}

	// check for source exist
	srcExist, err := SrcExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check src existence of %s in %s context: %w", url, context.Type, err)
	}

	if srcExist {
		logger.Info("src exists, we need to build it, then we need to call build(), then generateConfig(), then readProxy()")
	} else {
		// first prepare the src directory
		err := prepareSrcPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareSrcPath: %w", err)
		}
		logger.Warn("src doesn't exist, we need to download it using go-git then call build(), then generateConfig(), then readProxy()")
	}

	return nil
}
