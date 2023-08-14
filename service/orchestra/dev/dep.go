package dev

//
// Dep is the service that this service depends on
//

import (
	"fmt"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/net"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/config"
	"github.com/go-git/go-git/v5"
	"net/url"
	"os/exec"
	"path/filepath"
)

// The Dep is the dependency relied on by the service
type Dep struct {
	url     string
	context *Context // link back the Context where it's living
	cmd     *exec.Cmd
	done    chan error
}

func (dep *Dep) Url() string {
	return dep.url
}

func (dep *Dep) ConfigurationExist() (bool, error) {
	dataPath := dep.context.ConfigurationPath(dep.url)
	exists, err := path.FileExists(dataPath)
	if err != nil {
		return false, fmt.Errorf("path.FileExists('%s'): %w", dataPath, err)
	}
	return exists, nil
}

func (dep *Dep) prepareConfigurationPath() error {
	dir := filepath.Dir(dep.context.ConfigurationPath(dep.Url()))
	return preparePath(dir)
}

func (dep *Dep) prepareBinPath() error {
	dir := filepath.Dir(dep.context.BinPath(dep.Url()))
	return preparePath(dir)
}

func (dep *Dep) prepareSrcPath() error {
	dir := filepath.Dir(dep.context.SrcPath(dep.Url()))
	return preparePath(dir)
}

func (dep *Dep) BinExist() (bool, error) {
	dataPath := dep.context.BinPath(dep.Url())
	exists, err := path.FileExists(dataPath)
	if err != nil {
		return false, fmt.Errorf("path.FileExists('%s'): %w", dep.Url(), err)
	}
	return exists, nil
}

func (dep *Dep) SrcExist() (bool, error) {
	dataPath := dep.context.SrcPath(dep.Url())
	exists, err := path.FileExists(dataPath)
	if err != nil {
		return false, fmt.Errorf("path.FileExists('%s'): %w", dep.Url(), err)
	}
	return exists, nil
}

// GetServiceConfig returns the yaml config of the dependency as is
func (dep *Dep) GetServiceConfig() (*config.Service, error) {
	serviceConfig, err := dep.context.GetConfig(dep.Url())
	if err != nil {
		return nil, fmt.Errorf("config.GetConfig(%s): %w", dep.Url(), err)
	}

	return serviceConfig, nil
}

// SetServiceConfig updates the yaml of the proxy.
//
// It's needed for linting the dependency's destination server with the service that relies on it.
func (dep *Dep) SetServiceConfig(config *config.Service) error {
	return dep.context.SetConfig(dep.Url(), config)
}

// PrepareConfig creates the service.yml of the dependency.
// If it already exists, it skips.
// The prepared service.yml would be used for linting
func (dep *Dep) PrepareConfig(logger *log.Logger) error {
	exist, err := dep.ConfigurationExist()
	if err != nil {
		return fmt.Errorf("failed to check existence of %s in %s orchestra: %w", dep.url, dep.context.GetType(), err)
	}

	if exist {
		return nil
	}
	// first need to prepare the config
	err = dep.prepareConfigurationPath()
	if err != nil {
		return fmt.Errorf("prepareConfigurationPath: %w", err)
	}

	// check binary exists
	binExist, err := dep.BinExist()
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s orchestra: %w", dep.url, dep.context.GetType(), err)
	}

	if binExist {
		logger.Warn("todo: for the file when it's running we need to set the current orchestra as the same orchestra by setting .env")
		logger.Warn("todo: so that proxies or extensions will share the same orchestra")
		logger.Info("build config from the binary")

		err := dep.buildConfiguration(logger)
		if err != nil {
			return fmt.Errorf("buildConfiguration of %s: %w", dep.url, err)
		}
		logger.Info("config was built, read it")
		return nil
	} else {
		return fmt.Errorf("bin not found. call dep.Prepare()")
	}
}

// Prepare downloads the dependency if it wasn't.
// If the dependency binary exists, it will skip it.
// If it already exists, it skips.
// The prepared service.yml would be used for linting
func (dep *Dep) Prepare(logger *log.Logger) error {
	binExist, err := dep.BinExist()
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s orchestra: %w", dep.url, dep.context.GetType(), err)
	}

	if binExist {
		return nil
	}

	// first need to prepare the directory by creating it
	err = dep.prepareBinPath()
	if err != nil {
		return fmt.Errorf("prepareBinPath: %w", err)
	}

	// check for a source exist
	srcExist, err := dep.SrcExist()
	if err != nil {
		return fmt.Errorf("failed to check src existence of %s in %s orchestra: %w", dep.url, dep.context.GetType(), err)
	}
	if srcExist {
		logger.Info("src exists, we need to build it")
		err := dep.build(logger)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}

		return nil
	}

	// first prepare the src directory
	err = dep.prepareSrcPath()
	if err != nil {
		return fmt.Errorf("prepareSrcPath: %w", err)
	}

	err = dep.cloneSrc(logger)
	if err != nil {
		return fmt.Errorf("cloneSrc: %w", err)
	}

	err = dep.build(logger)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	return nil
}

// Run downloads the binary if it wasn't.
func (dep *Dep) Run(port uint64, logger *log.Logger) error {
	// check binary exists
	binExist, err := dep.BinExist()
	if err != nil {
		return fmt.Errorf("failed to check bin existence of %s in %s orchestra: %w", dep.url, dep.context.GetType(), err)
	}
	if !binExist {
		return fmt.Errorf("bin not found, call dep.Prepare()")
	}

	exist, err := dep.ConfigurationExist()
	if err != nil {
		return fmt.Errorf("failed to check existence of %s in %s orchestra: %w", dep.url, dep.context.GetType(), err)
	}
	if !exist {
		return fmt.Errorf("config not found. call dep.PrepareConfiguration")
	}

	used := net.IsPortUsed(dep.context.Host(), port)
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
}

// builds the application
func (dep *Dep) build(logger *log.Logger) error {
	srcUrl := dep.context.SrcPath(dep.Url())
	binUrl := dep.context.BinPath(dep.Url())

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
	binUrl := dep.context.BinPath(dep.Url())
	configFlag := fmt.Sprintf("--config=%s", dep.context.ConfigurationPath(dep.Url()))

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
			logger.Error("dep.orchestra.Close", "error", err)
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
func (dep *Dep) buildConfiguration(logger *log.Logger) error {
	binUrl := dep.context.BinPath(dep.Url())
	pathFlag := fmt.Sprintf("--path=%s", dep.context.ConfigurationPath(dep.Url()))
	urlFlag := fmt.Sprintf("--url=%s", dep.url)

	args, err := bindEnvs(dep.context, []string{"--build-config", pathFlag, urlFlag})
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
	srcUrl := dep.context.SrcPath(dep.Url())
	_, err = git.PlainClone(srcUrl, false, &git.CloneOptions{
		URL:      gitUrl,
		Progress: logger,
	})

	if err != nil {
		return fmt.Errorf("git.PlainClone --url %s --o %s: %w", gitUrl, srcUrl, err)
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

func bindEnvs(context *Context, args []string) ([]string, error) {
	envs := []string{context.EnvPath()}
	loadedEnvs, err := arg.GetEnvPaths()
	if err != nil {
		return []string{}, fmt.Errorf("failed to get env paths: %w", err)
	} else {
		envs = append(envs, loadedEnvs...)
	}

	return append(args, envs...), nil
}
