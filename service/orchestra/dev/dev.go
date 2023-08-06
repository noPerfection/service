// Package dev handles the dependencies in the development environment,
// which means it's in the current machine.
//
// The dependencies are including the extensions and proxies.
//
// How it works?
//
// The orchestra is set up. It checks the folder. And if they are not existing, it will create them.
// >> dev.Run(orchestra)
//
// then lets work on the extension.
// User is passing an extension url.
// The service is checking whether it exists in the data or not.
// If the service exists, it gets the yaml. And returns the config.
//
// If the service doesn't exist, it checks whether the service exists in the bin.
// If it exists, then it runs it with --build-config.
//
// Then, if the service doesn't exist in the bin, it checks the source.
// If the source exists, then it will call `go build`.
// Then call bin file with the generated files.
//
// Lastly, if a source doesn't exist, it will download the files from the repository using go-git.
// Then we build the binary.
// We generate config.
//
// Lastly, the service.Run() will make sure that all binaries exist.
// If not, then it will create them.
//
// -----------------------------------------------
// running the application will do the following.
// It checks the port of proxies is in use. If it's not, then it will call a run.
//
// Then it will call itself.
//
// The service will have a command to "shutdown" contexts. As well as "rebuild"
package dev

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/config/context"
	"github.com/ahmetson/service-lib/config/context/dev"
	"github.com/ahmetson/service-lib/config/env"
	"github.com/ahmetson/service-lib/server"
	"os"
)

type Context struct {
	config       *dev.Context
	controller   server.Interface
	serviceReady bool
	deps         map[string]*Dep
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

// New creates an orchestra including its directories.
func New(config context.Interface) (*Context, error) {
	if config.GetType() != context.DevContext {
		return nil, fmt.Errorf("ctx config is not a dev ctx. it's %s", config.GetType())
	}
	configContext, ok := config.(*dev.Context)
	if !ok {
		return nil, fmt.Errorf("can not convert ctx config into dev ctx")
	}

	ctx := &Context{
		config:     configContext,
		deps:       make(map[string]*Dep),
		controller: nil,
	}
	for _, path := range config.Paths() {
		if err := preparePath(path); err != nil {
			return ctx, fmt.Errorf("preparePath(%s): %w", path, err)
		}
	}

	if err := ctx.prepareEnv(); err != nil {
		return ctx, fmt.Errorf("prepareEnv: %w", err)
	}

	return ctx, nil
}

// prepareEnv writes the .env of the orchestra to share between dependencies.
// Call it after creating a path.
func (context *Context) prepareEnv() error {
	kv, err := key_value.NewFromInterface(context.config)
	if err != nil {
		return fmt.Errorf("prepareEnv: %w", err)
	}

	err = env.WriteEnv(kv, context.config.EnvPath())
	if err != nil {
		return fmt.Errorf("env.WriteEnv: %w", err)
	}

	return nil
}

// Dep returns the dependency from the orchestra by its url.
// Returns error, if the dependency wasn't found
func (context *Context) Dep(url string) (*Dep, error) {
	dep, ok := context.deps[url]
	if !ok {
		return nil, fmt.Errorf("the '%s' dependency not exists", url)
	}
	return dep, nil
}

// New dependency in the orchestra. If the dependency already exists, it will return an error.
// The created dependency will be added to the orchestra.
func (context *Context) New(url string) (*Dep, error) {
	_, ok := context.deps[url]
	if ok {
		return nil, fmt.Errorf("the '%s' dependency exists", url)
	}
	dep := &Dep{url: url, context: context, cmd: nil}
	context.deps[url] = dep
	return dep, nil
}
