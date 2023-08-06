package dev

//
// The orchester server has only one command.
//
// Close
// this command has no arguments. And when it's given, it will close all the dependencies it has
//

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/client"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/config"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/server"
)

// onClose closing all the dependencies in the orchester.
func (context *Context) onClose(request message.Request, logger *log.Logger, _ ...*client.ClientSocket) message.Reply {
	logger.Info("closing the orchester",
		"orchester type", context.config.GetType(),
		"service", context.config.GetUrl(),
		"todo", "close all dependencies if any",
		"todo", "close the main service",
		"goal", "exit the application")

	for _, dep := range context.deps {
		if dep.cmd == nil || dep.cmd.Process == nil {
			continue
		}

		// I expect that the killing process will release its resources as well.
		err := dep.cmd.Process.Kill()
		if err != nil {
			logger.Error("dep.cmd.Process.Kill", "error", err, "dep", dep.Url(), "command", "onClose")
			return request.Fail(fmt.Sprintf(`dep("%s").cmd.Process.Kill: %v`, dep.Url(), err))
		}
		logger.Info("dependency was closed", "url", dep.Url())
	}

	err := context.closeService(logger)
	if err != nil {
		return request.Fail(fmt.Sprintf("orchester.closeServer: %v", err))
	}
	// since we closed the service, for the orchester the service is not ready.
	// the service should call itself
	context.serviceReady = false

	logger.Info("dependencies were closed, service received a message to be closed as well")
	return request.Ok(key_value.Empty())
}

// onSetMainService marks the main service to be ready.
func (context *Context) onServiceReady(request message.Request, logger *log.Logger, _ ...*client.ClientSocket) message.Reply {
	logger.Info("onServiceReady", "type", "handler", "state", "enter")

	if context.serviceReady {
		return request.Fail("main service was set as true in the orchester")
	}
	context.serviceReady = true
	logger.Info("onServiceReady", "type", "handler", "state", "end")
	return request.Ok(key_value.Empty())
}

// Run the orchester in the background. If it failed to run, then return an error.
// The url request is the main service to which this orchester belongs too.
//
// The logger is the server logger as it is. The orchester will create its own logger from it.
func (context *Context) Run(logger *log.Logger) error {
	replier, err := server.SyncReplier(logger.Child("orchester"))
	if err != nil {
		return fmt.Errorf("server.SyncReplierType: %w", err)
	}

	config := config.InternalConfiguration(config.ContextName(context.config.GetUrl()))
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
	go func() {
		if err := context.controller.Run(); err != nil {
			logger.Fatal("orchester.server.Run: %w", err)
		}
	}()

	return nil
}

// Close sends a close signal to the orchester.
func (context *Context) Close(logger *log.Logger) error {
	if context.controller == nil {
		logger.Warn("skipping, since orchester.ControllerCategory is not initialised", "todo", "call orchester.Run()")
		return nil
	}
	contextName, contextPort := config.ClientUrlParameters(config.ContextName(context.config.GetUrl()))
	contextClient, err := client.NewReq(contextName, contextPort, logger)
	if err != nil {
		logger.Error("client.NewReq", "error", err)
		return fmt.Errorf("close the service by hand. client.NewReq: %w", err)
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

	// release the orchester parameters
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
		logger.Warn("orchester.ControllerCategory is not initialised", "todo", "call orchester.Run()")
		return nil
	}
	contextName, contextPort := config.ClientUrlParameters(config.ContextName(context.config.GetUrl()))
	contextClient, err := client.NewReq(contextName, contextPort, logger)
	if err != nil {
		return fmt.Errorf("close the service by hand. client.NewReq: %w", err)
	}

	closeRequest := &message.Request{
		Command:    "service-ready",
		Parameters: key_value.Empty(),
	}

	_, err = contextClient.RequestRemoteService(closeRequest)
	if err != nil {
		return fmt.Errorf("close the service by hand. contextClient.RequestRemoteService: %w", err)
	}

	// release the orchester parameters
	err = contextClient.Close()
	if err != nil {
		return fmt.Errorf("contextClient.Close: %w", err)
	}

	return nil
}

// CloseService sends a close signal to the manager.
func (context *Context) closeService(logger *log.Logger) error {
	if !context.serviceReady {
		logger.Warn("!orchester.serviceReady")
		return nil
	}
	logger.Info("main service is linted to the orchester. send a signal to main service to be closed")

	contextName, contextPort := config.ClientUrlParameters(config.ManagerName(context.config.GetUrl()))
	contextClient, err := client.NewReq(contextName, contextPort, logger)
	if err != nil {
		return fmt.Errorf("close the service by hand. client.NewReq: %w", err)
	}

	closeRequest := &message.Request{
		Command:    "close",
		Parameters: key_value.Empty(),
	}

	_, err = contextClient.RequestRemoteService(closeRequest)
	if err != nil {
		return fmt.Errorf("close the service by hand. contextClient.RequestRemoteService: %w", err)
	}

	// release the orchester parameters
	err = contextClient.Close()
	if err != nil {
		return fmt.Errorf("contextClient.Close: %w", err)
	}

	return nil
}
