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
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/configuration/env"
	"github.com/ahmetson/service-lib/log"
	"github.com/go-git/go-git/v5" // with go modules disabled
	"net/url"
	"os"
	"os/exec"
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

	if err := prepareEnv(context); err != nil {
		return fmt.Errorf("prepareEnv: %w", err)
	}

	return nil
}

// prepareEnv writes the .env of the context to share between dependencies
func prepareEnv(context *configuration.Context) error {
	kv, err := key_value.NewFromInterface(context)
	if err != nil {
		return fmt.Errorf("prepareEnv: %w", err)
	}

	envPath := EnvPath(context)

	err = env.WriteEnv(kv, envPath)
	if err != nil {
		return fmt.Errorf("env.WriteEnv: %w", err)
	}

	return nil
}

// EnvPath is the shared configurations between dependencies
func EnvPath(context *configuration.Context) string {
	return path.Join(context.Data, ".env")
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
	return path.Join(context.Src, url)
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

// ReadServiceConfiguration returns the yaml configuration of the dependency as is
func ReadServiceConfiguration(context *configuration.Context, url string) (configuration.Service, error) {
	configUrl := ConfigurationPath(context, url)
	service, err := configuration.ReadService(configUrl)
	if err != nil {
		return configuration.Service{}, fmt.Errorf("configuration.ReadService of %s: %w", configUrl, err)
	}

	return service, nil
}

// WriteServiceConfiguration updates the yaml of the proxy.
//
// It's needed for linting the dependency's destination controller with the service that relies on it.
func WriteServiceConfiguration(context *configuration.Context, url string, config configuration.Service) error {
	configUrl := ConfigurationPath(context, url)
	return configuration.WriteService(configUrl, config)
}

// PrepareConfiguration creates the service.yml of the dependency.
// If it already exists, it skips.
// The prepared service.yml would be used for linting
func PrepareConfiguration(context *configuration.Context, url string, logger *log.Logger) error {
	exist, err := ConfigurationExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check existence of %s in %s context: %w", url, context.Type, err)
	}

	if exist {
		return nil
	} else {
		// first need to prepare the configuration
		err := prepareConfigurationPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareConfigurationPath: %w", err)
		}
	}

	// check is binary exist
	binExist, err := BinExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s context: %w", url, context.Type, err)
	}

	if binExist {
		logger.Warn("todo: for the file when it's running we need to set the current context as the same context by setting .env")
		logger.Warn("todo: so that proxies or extensions will share the same context")

		logger.Info("build configuration from the binary")
		err := buildConfiguration(context, url, logger)
		if err != nil {
			return fmt.Errorf("buildConfiguration of %s: %w", url, err)
		}
		logger.Info("configuration was built, read it")
		return PrepareConfiguration(context, url, logger)
	} else {
		// first need to prepare the directory
		err := prepareBinPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareBinPath: %w", err)
		}
	}

	// check for source exist
	srcExist, err := SrcExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check src existence of %s in %s context: %w", url, context.Type, err)
	}

	if srcExist {
		logger.Info("src exists, we need to build it")
		err := build(context, url, logger)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		logger.Info("file was built. generate the configuration file")
		return PrepareConfiguration(context, url, logger)
	} else {
		// first prepare the src directory
		err := prepareSrcPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareSrcPath: %w", err)
		}

		logger.Warn("src doesn't exist, try to clone the source code")
		err = cloneSrc(context, url, logger)
		if err != nil {
			return fmt.Errorf("cloneSrc: %w", err)
		} else {
			logger.Info("prepare again to build the source")
			return PrepareConfiguration(context, url, logger)
		}
	}
}

// PrepareService runs the service if it wasn't running
func PrepareService(context *configuration.Context, url string, port uint64, logger *log.Logger) error {
	// check is binary exist
	binExist, err := BinExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s context: %w", url, context.Type, err)
	}

	if binExist {
		used := configuration.IsPortUsed(context.Host(), port)
		if used {
			logger.Info("service is launched already", "url", url, "port", port)
			return nil
		} else {
			err := start(context, url, logger)
			if err != nil {
				return fmt.Errorf("failed to start proxy: %w", err)
			}
			return nil
		}
	} else {
		// first need to prepare the directory
		err := prepareBinPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareBinPath: %w", err)
		}
	}

	// check for source exist
	srcExist, err := SrcExist(context, url)
	if err != nil {
		return fmt.Errorf("failed to check src existence of %s in %s context: %w", url, context.Type, err)
	}

	if srcExist {
		logger.Info("src exists, we need to build it")
		err := build(context, url, logger)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		logger.Info("file was built.")
		return PrepareService(context, url, port, logger)
	} else {
		// first prepare the src directory
		err := prepareSrcPath(context, url)
		if err != nil {
			return fmt.Errorf("prepareSrcPath: %w", err)
		}

		logger.Warn("src doesn't exist, try to clone the source code")
		err = cloneSrc(context, url, logger)
		if err != nil {
			return fmt.Errorf("cloneSrc: %w", err)
		} else {
			logger.Info("prepare again to build the source")
			return PrepareService(context, url, port, logger)
		}
	}
}

func bindEnvs(context *configuration.Context, args []string) ([]string, error) {
	envs := []string{EnvPath(context)}
	loadedEnvs, err := argument.GetEnvPaths()
	if err != nil {
		return []string{}, fmt.Errorf("failed to get env paths: %w", err)
	} else {
		envs = append(envs, loadedEnvs...)
	}

	return append(args, envs...), nil
}

// builds the application
func build(context *configuration.Context, url string, logger *log.Logger) error {
	srcUrl := SrcPath(context, url)
	binUrl := BinPath(context, url)

	logger.Info("building", "src", srcUrl, "bin", binUrl)

	err := cleanBuild(srcUrl, logger)
	if err != nil {
		return fmt.Errorf("cleanBuild: %w", err)
	} else {
		logger.Info("go mod tidy was called in ", "source", srcUrl)
	}

	cmd := exec.Command("go", "build", "-o", binUrl)
	cmd.Stdout = logger
	cmd.Dir = srcUrl
	cmd.Stderr = logger
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Run: %w", err)
	}
	return nil
}

// start is run without attachment
func start(context *configuration.Context, url string, logger *log.Logger) error {
	binUrl := BinPath(context, url)
	configFlag := fmt.Sprintf("--configuration=%s", ConfigurationPath(context, url))

	args, err := bindEnvs(context, []string{configFlag})
	if err != nil {
		return fmt.Errorf("bindEnvs to args: %w", err)
	}

	logger.Info("running", "command", binUrl, "arguments", args)

	cmd := exec.Command(binUrl, args...)
	cmd.Stdout = logger
	cmd.Stderr = logger
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}

	return nil
}

// calls `go mod tidy`
func cleanBuild(srcUrl string, logger *log.Logger) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout = logger
	cmd.Dir = srcUrl
	cmd.Stderr = logger
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Run: %w", err)
	}

	return nil
}

// calls `go mod tidy`
func buildConfiguration(context *configuration.Context, url string, logger *log.Logger) error {
	binUrl := BinPath(context, url)
	pathFlag := fmt.Sprintf("--path=%s", ConfigurationPath(context, url))
	urlFlag := fmt.Sprintf("--url=%s", url)

	args, err := bindEnvs(context, []string{"--build-configuration", pathFlag, urlFlag})
	if err != nil {
		return fmt.Errorf("bindEnvs to args: %w", err)
	}
	logger.Info("executing", "bin", binUrl, "arguments", args)

	cmd := exec.Command(binUrl, args...)

	cmd.Stdout = logger
	cmd.Stderr = logger
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Run: %w", err)
	}

	return nil
}

func convertToGitUrl(rawUrl string) (string, error) {
	URL, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	URL.Scheme = "https"

	println("url", URL, "protocol", URL.Scheme)
	return URL.String() + ".git", nil
}

func cloneSrc(context *configuration.Context, url string, logger *log.Logger) error {
	gitUrl, err := convertToGitUrl(url)
	if err != nil {
		return fmt.Errorf("convertToGitUrl of %s: %w", url, err)
	}
	srcUrl := SrcPath(context, url)
	_, err = git.PlainClone(srcUrl, false, &git.CloneOptions{
		URL:      gitUrl,
		Progress: logger,
	})

	if err != nil {
		return fmt.Errorf("git.PlainClone --url %s --o %s: %w", gitUrl, srcUrl, err)
	}

	return nil
}
