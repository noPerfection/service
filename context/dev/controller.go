package dev

//
// The context controller has only one command.
//
// close
// this command has no arguments. and when it's given it will close all the dependencies it has
//

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
)

// onClose closing all the dependencies in the context.
func (context *Context) onClose(request message.Request, logger *log.Logger, _ ...*remote.ClientSocket) message.Reply {
	logger.Info("closing the context",
		"context type", context.config.Type,
		"service", context.config.GetUrl(),
		"todo", "close all dependencies if any",
		"todo", "close the main service",
		"goal", "exit the application")

	for _, dep := range context.deps {
		if dep.cmd == nil || dep.cmd.Process == nil {
			continue
		}

		// I expect that killing the process will release its resources as well.
		err := dep.cmd.Process.Kill()
		if err != nil {
			logger.Error("dep.cmd.Process.Kill", "error", err, "dep", dep.Url(), "command", "onClose")
			return request.Fail(fmt.Sprintf(`dep("%s").cmd.Process.Kill: %v`, dep.Url(), err))
		}
		logger.Info("dependency was closed", "url", dep.Url())
	}

	err := context.closeService(logger)
	if err != nil {
		return request.Fail(fmt.Sprintf("context.closeServer: %v", err))
	}
	// since we closed the service, for the context the service is not ready.
	// the service should call itself
	context.serviceReady = false

	logger.Info("dependencies were closed, service received a message to be closed as well")
	return request.Ok(key_value.Empty())
}

// onSetMainService marks the main service to be ready.
func (context *Context) onServiceReady(request message.Request, logger *log.Logger, _ ...*remote.ClientSocket) message.Reply {
	logger.Info("onServiceReady", "type", "handler", "state", "enter")

	if context.serviceReady {
		return request.Fail("main service was set as true in the context")
	}
	context.serviceReady = true
	logger.Info("onServiceReady", "type", "handler", "state", "end")
	return request.Ok(key_value.Empty())
}

// Run the context in the background. If it failed to run, then return an error.
// The url parameter is the main service to which this context belongs too.
//
// The logger is the server logger as is. The context will create its own logger from it.
func (context *Context) Run(logger *log.Logger) error {
	replier, err := controller.SyncReplier(logger.Child("context"))
	if err != nil {
		return fmt.Errorf("controller.SyncReplierType: %w", err)
	}

	config := configuration.InternalConfiguration(configuration.ContextName(context.config.GetUrl()))
	replier.AddConfig(config, context.config.GetUrl())

	closeRoute := command.NewRoute("close", context.onClose)
	serviceReadyRoute := command.NewRoute("service-ready", context.onServiceReady)
	err = replier.AddRoute(closeRoute)
	if err != nil {
		return fmt.Errorf(`replier.AddRoute("close"): %w`, err)
	}
	err = replier.AddRoute(serviceReadyRoute)
	if err != nil {
		return fmt.Errorf(`replier.AddRoute("service-ready"): %w`, err)
	}

	context.controller = replier
	go context.controller.Run()

	return nil
}

// Close sends a close signal to the context.
func (context *Context) Close(logger *log.Logger) error {
	if context.controller == nil {
		logger.Warn("skipping, since context.Controller is not initialised", "todo", "call context.Run()")
		return nil
	}
	contextName, contextPort := configuration.ClientUrlParameters(configuration.ContextName(context.config.GetUrl()))
	contextClient, err := remote.NewReq(contextName, contextPort, logger)
	if err != nil {
		logger.Error("remote.NewReq", "error", err)
		return fmt.Errorf("close the service by hand. remote.NewReq: %w", err)
	}

	closeRequest := &message.Request{
		Command:    "close",
		Parameters: key_value.Empty(),
	}

	_, err = contextClient.RequestRemoteService(closeRequest)
	if err != nil {
		logger.Error("contextClient.RequestRemoteService", "error", err)
		return fmt.Errorf("close the service by hand. contextClient.RequestRemoteService: %w", err)
	}

	// release the context parameters
	err = contextClient.Close()
	if err != nil {
		logger.Error("contextClient.Close", "error", err)
		return fmt.Errorf("contextClient.Close: %w", err)
	}

	return nil
}

// ServiceReady sends a signal marking that the main service is ready.
func (context *Context) ServiceReady(logger *log.Logger) error {
	if context.controller == nil {
		logger.Warn("context.Controller is not initialised", "todo", "call context.Run()")
		return nil
	}
	contextName, contextPort := configuration.ClientUrlParameters(configuration.ContextName(context.config.GetUrl()))
	contextClient, err := remote.NewReq(contextName, contextPort, logger)
	if err != nil {
		return fmt.Errorf("close the service by hand. remote.NewReq: %w", err)
	}

	closeRequest := &message.Request{
		Command:    "service-ready",
		Parameters: key_value.Empty(),
	}

	_, err = contextClient.RequestRemoteService(closeRequest)
	if err != nil {
		return fmt.Errorf("close the service by hand. contextClient.RequestRemoteService: %w", err)
	}

	// release the context parameters
	err = contextClient.Close()
	if err != nil {
		return fmt.Errorf("contextClient.Close: %w", err)
	}

	return nil
}

// CloseService sends a close signal to the manager.
func (context *Context) closeService(logger *log.Logger) error {
	if !context.serviceReady {
		logger.Warn("!context.serviceReady")
		return nil
	}
	logger.Info("main service is linted to the context. send a signal to main service to be closed")

	contextName, contextPort := configuration.ClientUrlParameters(configuration.ManagerName(context.config.GetUrl()))
	contextClient, err := remote.NewReq(contextName, contextPort, logger)
	if err != nil {
		return fmt.Errorf("close the service by hand. remote.NewReq: %w", err)
	}

	closeRequest := &message.Request{
		Command:    "close",
		Parameters: key_value.Empty(),
	}

	_, err = contextClient.RequestRemoteService(closeRequest)
	if err != nil {
		return fmt.Errorf("close the service by hand. contextClient.RequestRemoteService: %w", err)
	}

	// release the context parameters
	err = contextClient.Close()
	if err != nil {
		return fmt.Errorf("contextClient.Close: %w", err)
	}

	return nil
}
