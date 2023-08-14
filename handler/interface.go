package handler

import (
	"fmt"
	client "github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/config/service"
)

// Interface of the handler. All controllers have it
//
// The interface that it accepts is the *client.ClientSocket from the
// "github.com/ahmetson/client-lib" package.
type Interface interface {
	// AddConfig adds the parameters of the handler from the config
	AddConfig(controller *service.Controller, serviceUrl string)

	// AddExtensionConfig adds the config of the extension that the handler depends on
	AddExtensionConfig(extension *service.Extension)

	// RequireExtension marks the extensions that this handler depends on.
	// Before running, the required extension should be added from the config.
	// Otherwise, handler won't run.
	RequireExtension(name string)

	// RequiredExtensions returns the list of extension names required by this handler
	RequiredExtensions() []string

	// AddRoute registers a new command and it's handlers for this handler
	AddRoute(route *command.Route) error

	// ControllerType returns the type of the handler
	ControllerType() service.ControllerType

	// Close the handler if it's running. If it's not running, then do nothing
	Close() error

	Run() error
}

// Does nothing, simply returns the data
var anyHandler = func(request message.Request, _ *log.Logger, _ ...*client.ClientSocket) message.Reply {
	replyParameters := key_value.Empty()
	replyParameters.Set("command", request.Command)

	reply := request.Ok(replyParameters)
	return reply
}

// AnyRoute makes the given handler as the source of the proxy.
// It means, it will add command.Any to call the proxy.
func AnyRoute(sourceController Interface) error {
	route := command.NewRoute(command.Any, anyHandler)

	if err := sourceController.AddRoute(route); err != nil {
		return fmt.Errorf("failed to add any route into the handler: %w", err)
	}
	return nil
}

func requiredMetadata() []string {
	return []string{"Identity", "pub_key"}
}
