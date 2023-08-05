package controller

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration/service"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
)

// Interface of the controller. All controllers have it
//
// The interface that it accepts is the *remote.ClientSocket from the
// "github.com/ahmetson/service-lib/remote" package.
type Interface interface {
	// AddConfig adds the parameters of the controller from the configuration
	AddConfig(controller *service.Controller, serviceUrl string)

	// AddExtensionConfig adds the configuration of the extension that the controller depends on
	AddExtensionConfig(extension *service.Extension)

	// RequireExtension marks the extensions that this controller depends on.
	// Before running, the required extension should be added from the configuration.
	// Otherwise, controller won't run.
	RequireExtension(name string)

	// RequiredExtensions returns the list of extension names required by this controller
	RequiredExtensions() []string

	// AddRoute registers a new command and it's handlers for this controller
	AddRoute(route *command.Route) error

	// ControllerType returns the type of the controller
	ControllerType() service.ControllerType

	// Close the controller if it's running. If it's not running, then do nothing
	Close() error

	Run() error
}

// Does nothing, simply returns the data
var anyHandler = func(request message.Request, _ *log.Logger, _ ...*remote.ClientSocket) message.Reply {
	replyParameters := key_value.Empty()
	replyParameters.Set("command", request.Command)

	reply := request.Ok(replyParameters)
	return reply
}

// AnyRoute makes the given controller as the source of the proxy.
// It means, it will add command.Any to call the proxy.
func AnyRoute(sourceController Interface) error {
	route := command.NewRoute(command.Any, anyHandler)

	if err := sourceController.AddRoute(route); err != nil {
		return fmt.Errorf("failed to add any route into the controller: %w", err)
	}
	return nil
}

func requiredMetadata() []string {
	return []string{"Identity", "pub_key"}
}
