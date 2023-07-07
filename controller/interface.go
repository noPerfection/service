package controller

import (
	"github.com/Seascape-Foundation/sds-service-lib/communication/command"
	"github.com/Seascape-Foundation/sds-service-lib/configuration"
)

// Interface of the controller. All controllers have it
//
// The interface that it accepts is the *remote.ClientSocket from the
// "github.com/Seascape-Foundation/sds-service-lib/remote" package.
type Interface interface {
	// AddConfig adds the parameters of the controller from the configuration
	AddConfig(controller *configuration.Controller)

	// AddExtensionConfig adds the configuration of the extension that the controller depends on
	AddExtensionConfig(extension *configuration.Extension)

	// RequireExtension marks the extensions that this controller depends on.
	// Before running, the required extension should be added from the configuration.
	// Otherwise, controller won't run.
	RequireExtension(name string)

	// RequiredExtensions returns the list of extension names required by this controller
	RequiredExtensions() []string

	RegisterCommand(name command.Name, handler command.HandleFunc)

	Run() error
}
