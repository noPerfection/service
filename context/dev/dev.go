// Package dev handles the dependencies in the development environment,
// which means it's in the current machine.
//
// The dependencies are including the extensions and proxies.
//
// How it works?
//
// The context is set up. It checks the folder. And if they are not existing, it will create them.
// >> dev.Prepare(context)
//
// then lets work on the extension.
// User is passing an extension url.
// The service is checking whether it exists in the data or not.
// If the service exists, it gets the yaml. And returns the configuration.
//
// If the service doesn't exist, it checks whether the service exists in the bin.
// If it exists, then it runs it with --build-configuration.
//
// Then, if the service doesn't exist in the bin, it checks the source.
// If the source exists, then it will call `go build`.
// Then call bin file with the generated files.
//
// Lastly, if a source doesn't exist, it will download the files from the repository using go-git.
// Then we build the binary.
// We generate configuration.
//
// Lastly, the service.Prepare() will make sure that all binaries exist.
// If not, then it will create them.
//
// -----------------------------------------------
// running the application will do the following.
// It checks is the port of proxies are in use. If it's not, then it will call a run.
//
// Then it will call itself.
//
// The service will have a command to "shutdown" contexts. As well as "rebuild"
package dev

import (
	"errors"
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/configuration/context"
	"github.com/ahmetson/service-lib/configuration/context/dev"
	"github.com/ahmetson/service-lib/configuration/env"
	"github.com/ahmetson/service-lib/configuration/network"
	"github.com/ahmetson/service-lib/configuration/service"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/go-git/go-git/v5" // with go modules disabled
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
)

type Context struct {
	config       *context.Context
	controller   controller.Interface
	serviceReady bool
	deps         map[string]*Dep
}

// The Dep is the dependency relied on by the service
type Dep struct {
	url     string
	context *Context // link back the Context where it's living
	cmd     *exec.Cmd
	done    chan error
}

// Creates the directory for the data at the given path.
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

// New creates a context including its directories.
func New(config *context.Context) (*Context, error) {
	context := &Context{
		config:     config,
		deps:       make(map[string]*Dep),
		controller: nil,
	}
	for _, path := range config.Paths() {
		if err := preparePath(path); err != nil {
			return context, fmt.Errorf("preparePath(%s): %w", path, err)
		}
	}

	if err := context.prepareEnv(); err != nil {
		return context, fmt.Errorf("prepareEnv: %w", err)
	}

	return context, nil
}

// prepareEnv writes the .env of the context to share between dependencies.
// Call it after creating a path.
func (context *Context) prepareEnv() error {
	kv, err := key_value.NewFromInterface(context.config)
	if err != nil {
		return fmt.Errorf("prepareEnv: %w", err)
	}

	err = env.WriteEnv(kv, context.EnvPath())
	if err != nil {
		return fmt.Errorf("env.WriteEnv: %w", err)
	}

	return nil
}

// Dep returns the dependency from the context by its url.
// Returns error, if the dependency wasn't found
func (context *Context) Dep(url string) (*Dep, error) {
	dep, ok := context.deps[url]
	if !ok {
		return nil, fmt.Errorf("the '%s' dependency not exists", url)
	}
	return dep, nil
}

// New dependency in the context. If the dependency already exists, it will return an error.
// The created dependency will be added to the context.
func (context *Context) New(url string) (*Dep, error) {
	_, ok := context.deps[url]
	if ok {
		return nil, fmt.Errorf("the '%s' dependency exists", url)
	}
	dep := &Dep{url: url, context: context, cmd: nil}
	context.deps[url] = dep
	return dep, nil
}

// EnvPath is the shared configurations between dependencies
func (context *Context) EnvPath() string {
	return filepath.Join(context.config.Data, ".env")
}

func (dep *Dep) Url() string {
	return dep.url
}

// ConfigurationPath returns configuration url in the context's data
func (dep *Dep) ConfigurationPath() string {
	fileName := configuration.UrlToFileName(dep.context.config.GetUrl())
	return filepath.Join(dep.context.config.Data, dep.url, fileName+".yml")
}

func (dep *Dep) ConfigurationExist() (bool, error) {
	dataPath := dep.ConfigurationPath()
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

func (dep *Dep) prepareConfigurationPath() error {
	dir := filepath.Dir(dep.ConfigurationPath())
	return preparePath(dir)
}

func (dep *Dep) prepareBinPath() error {
	dir := filepath.Dir(dep.BinPath())
	return preparePath(dir)
}

func (dep *Dep) prepareSrcPath() error {
	dir := filepath.Dir(dep.SrcPath())
	return preparePath(dir)
}

func (dep *Dep) BinExist() (bool, error) {
	dataPath := dep.BinPath()
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

func (dep *Dep) SrcPath() string {
	return filepath.Join(dep.context.config.Src, dep.url)
}

func (dep *Dep) SrcExist() (bool, error) {
	dataPath := dep.SrcPath()
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

// Configuration returns the yaml configuration of the dependency as is
func (dep *Dep) Configuration() (*service.Service, error) {
	configUrl := dep.ConfigurationPath()
	service, err := dev.ReadService(configUrl)
	if err != nil {
		return nil, fmt.Errorf("configuration.ReadService of %s: %w", configUrl, err)
	}

	return service, nil
}

// SetConfiguration updates the yaml of the proxy.
//
// It's needed for linting the dependency's destination controller with the service that relies on it.
func (dep *Dep) SetConfiguration(config *service.Service) error {
	configUrl := dep.ConfigurationPath()
	return dev.WriteService(configUrl, config)
}

// PrepareConfiguration creates the service.yml of the dependency.
// If it already exists, it skips.
// The prepared service.yml would be used for linting
func (dep *Dep) PrepareConfiguration(logger *log.Logger) error {
	exist, err := dep.ConfigurationExist()
	if err != nil {
		return fmt.Errorf("failed to check existence of %s in %s context: %w", dep.url, dep.context.config.Type, err)
	}

	if exist {
		return nil
	} else {
		// first need to prepare the configuration
		err := dep.prepareConfigurationPath()
		if err != nil {
			return fmt.Errorf("prepareConfigurationPath: %w", err)
		}
	}

	// check binary exists
	binExist, err := dep.BinExist()
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s context: %w", dep.url, dep.context.config.Type, err)
	}

	if binExist {
		logger.Warn("todo: for the file when it's running we need to set the current context as the same context by setting .env")
		logger.Warn("todo: so that proxies or extensions will share the same context")
		logger.Info("build configuration from the binary")

		err := dep.buildConfiguration(logger)
		if err != nil {
			return fmt.Errorf("buildConfiguration of %s: %w", dep.url, err)
		}
		logger.Info("configuration was built, read it")
		return dep.PrepareConfiguration(logger)
	} else {
		// first need to prepare the directory
		err := dep.prepareBinPath()
		if err != nil {
			return fmt.Errorf("prepareBinPath: %w", err)
		}
	}

	// check for source exist
	srcExist, err := dep.SrcExist()
	if err != nil {
		return fmt.Errorf("failed to check src existence of %s in %s context: %w", dep.url, dep.context.config.Type, err)
	}

	if srcExist {
		logger.Info("src exists, we need to build it")
		err := dep.build(logger)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		logger.Info("file was built. generate the configuration file")
		return dep.PrepareConfiguration(logger)
	} else {
		// first prepare the src directory
		err := dep.prepareSrcPath()
		if err != nil {
			return fmt.Errorf("prepareSrcPath: %w", err)
		}

		logger.Warn("src doesn't exist, try to clone the source code")
		err = dep.cloneSrc(logger)
		if err != nil {
			return fmt.Errorf("cloneSrc: %w", err)
		} else {
			logger.Info("prepare again to build the source")
			return dep.PrepareConfiguration(logger)
		}
	}
}

// Prepare downloads the binary if it wasn't.
func (dep *Dep) Prepare(port uint64, logger *log.Logger) error {
	// check binary exists
	binExist, err := dep.BinExist()
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s context: %w", dep.url, dep.context.config.Type, err)
	}

	if binExist {
		used := network.IsPortUsed(dep.context.config.Host(), port)
		if used {
			logger.Info("service is launched already", "url", dep.url, "port", port)
			return nil
		} else {
			err := dep.start(logger)
			if err != nil {
				return fmt.Errorf("failed to start proxy: %w", err)
			}
			return nil
		}
	} else {
		// first need to prepare the directory
		err := dep.prepareBinPath()
		if err != nil {
			return fmt.Errorf("prepareBinPath: %w", err)
		}
	}

	// check for source exist
	srcExist, err := dep.SrcExist()
	if err != nil {
		return fmt.Errorf("failed to check src existence of %s in %s context: %w", dep.url, dep.context.config.Type, err)
	}

	if srcExist {
		logger.Info("src exists, we need to build it")
		err := dep.build(logger)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		logger.Info("file was built.")
		return dep.Prepare(port, logger)
	} else {
		// first prepare the src directory
		err := dep.prepareSrcPath()
		if err != nil {
			return fmt.Errorf("prepareSrcPath: %w", err)
		}

		logger.Warn("src doesn't exist, try to clone the source code")
		err = dep.cloneSrc(logger)
		if err != nil {
			return fmt.Errorf("cloneSrc: %w", err)
		} else {
			logger.Info("prepare again to build the source")
			return dep.Prepare(port, logger)
		}
	}
}

func bindEnvs(context *Context, args []string) ([]string, error) {
	envs := []string{context.EnvPath()}
	loadedEnvs, err := argument.GetEnvPaths()
	if err != nil {
		return []string{}, fmt.Errorf("failed to get env paths: %w", err)
	} else {
		envs = append(envs, loadedEnvs...)
	}

	return append(args, envs...), nil
}

// builds the application
func (dep *Dep) build(logger *log.Logger) error {
	srcUrl := dep.SrcPath()
	binUrl := dep.BinPath()

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

// start is run without an attachment
func (dep *Dep) start(logger *log.Logger) error {
	binUrl := dep.BinPath()
	configFlag := fmt.Sprintf("--configuration=%s", dep.ConfigurationPath())

	args, err := bindEnvs(dep.context, []string{configFlag})
	if err != nil {
		return fmt.Errorf("bindEnvs to args: %w", err)
	}

	logger.Info("running", "command", binUrl, "arguments", args)

	dep.done = make(chan error, 1)
	dep.onEnd(logger)

	cmd := exec.Command(binUrl, args...)
	cmd.Stdout = logger
	cmd.Stderr = logger
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}
	dep.cmd = cmd
	dep.wait(logger)

	return nil
}

// Call it before starting the dependency with os/exec.Start
func (dep *Dep) onEnd(logger *log.Logger) {
	go func() {
		err := <-dep.done
		if err != nil {
			logger.Error("dependency ended with error", "error", err, "dep", dep.Url())
		} else {
			logger.Info("dependency ended successfully", "dep", dep.Url())
		}
		dep.cmd = nil
		err = dep.context.Close(logger)
		if err != nil {
			logger.Error("dep.context.Close", "error", err)
		}
	}()
}

// wait until the dependency is not exiting
func (dep *Dep) wait(logger *log.Logger) {
	go func() {
		logger.Info("waiting for dep to end", "dep", dep.Url())
		err := dep.cmd.Wait()
		logger.Error("dependency closed itself", "dep", dep.Url(), "error", err)
		dep.done <- err
	}()
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
func (dep *Dep) buildConfiguration(logger *log.Logger) error {
	binUrl := dep.BinPath()
	pathFlag := fmt.Sprintf("--path=%s", dep.ConfigurationPath())
	urlFlag := fmt.Sprintf("--url=%s", dep.url)

	args, err := bindEnvs(dep.context, []string{"--build-configuration", pathFlag, urlFlag})
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

func (dep *Dep) cloneSrc(logger *log.Logger) error {
	gitUrl, err := convertToGitUrl(dep.url)
	if err != nil {
		return fmt.Errorf("convertToGitUrl of %s: %w", dep.url, err)
	}
	srcUrl := dep.SrcPath()
	_, err = git.PlainClone(srcUrl, false, &git.CloneOptions{
		URL:      gitUrl,
		Progress: logger,
	})

	if err != nil {
		return fmt.Errorf("git.PlainClone --url %s --o %s: %w", gitUrl, srcUrl, err)
	}

	return nil
}
