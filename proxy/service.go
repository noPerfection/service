// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"sync"
)

// Service defines the parameters of the proxy service
type Service struct {
	// configuration of the whole app with the configuration engine
	configuration *configuration.Config
	// source controllers that gets the messages
	source controller.Interface
	// Controller that handles the requests and redirects to the destination.
	Controller *Controller
	logger     log.Logger
}

// SourceName of this type should be listed within the controllers in the configuration
const SourceName = "source"

// DestinationName of this type should be listed within the controllers in the configuration
const DestinationName = "destination"

// extension creates the configuration of the proxy controller.
// The proxy controller itself is added as the extension to the source controllers,
// to the request handlers and to the reply handlers.
func extension() *configuration.Extension {
	return configuration.NewInternalExtension(ControllerName)
}

func validateConfiguration(service configuration.Service) error {
	if len(service.Controllers) < 2 {
		return fmt.Errorf("not enough controllers were given. atleast 'source' and 'destination' should be")
	}

	sourceFound := false
	destinationFound := false
	for _, c := range service.Controllers {
		if c.Name == SourceName {
			sourceFound = true
		} else if c.Name == DestinationName {
			destinationFound = true
		}
	}

	if !sourceFound {
		return fmt.Errorf("proxy service '%s' in seascape.yml doesn't have '%s' controller", service.Name, SourceName)
	}

	if !destinationFound {
		return fmt.Errorf("proxy service '%s' in seascape.yml doesn't have '%s' controller", service.Name, DestinationName)
	}

	return nil
}

// registerNonSources registers the controller instances as the destination.
// it skips the SourceName named controllers as the destination.
func registerNonSources(controllers []configuration.Controller, proxyController *Controller) error {
	for _, c := range controllers {
		if c.Name == SourceName {
			continue
		}

		proxyController.RegisterDestination(&c)
	}

	return nil
}

// New proxy service along with its controller.
func New(config *configuration.Config, logger log.Logger) *Service {
	controller := newController(logger.Child("controller"))

	service := Service{
		configuration: config,
		source:        nil,
		Controller:    controller,
		logger:        logger,
	}

	return &service
}

func (service *Service) Prepare() error {
	serviceConf := service.configuration.Service
	if serviceConf.Type != configuration.ProxyType {
		return fmt.Errorf("service type in the configuration is not Independent. It's '%s'", serviceConf.Type)
	}
	if err := validateConfiguration(serviceConf); err != nil {
		return fmt.Errorf("validateConfiguration: %w", err)
	}

	err := registerNonSources(serviceConf.Controllers, service.Controller)
	if err != nil {
		return fmt.Errorf("registerNonSources: %w", err)
	}

	// validate the controllers
	// validate the extensions that the controllers required extensions are in the source.

	proxyExtension := extension()

	// Run the sources
	// add the extensions required by the source controller
	requiredExtensions := service.source.RequiredExtensions()
	for _, name := range requiredExtensions {
		extension, err := service.configuration.Service.GetExtension(name)
		if err != nil {
			log.Fatal("extension required by the controller doesn't exist in the configuration", "error", err)
		}

		service.source.AddExtensionConfig(extension)
	}

	// The proxy adds itself as the extension to the sources
	service.source.RequireExtension(proxyExtension.Name)
	service.source.AddExtensionConfig(proxyExtension)

	return nil
}

// SetDefaultSource creates a source controller of the given type.
//
// It loads the source name automatically.
func (service *Service) SetDefaultSource(controllerType configuration.Type) error {
	// todo move the validation to the service.ValidateTypes() function
	var source controller.Interface
	if controllerType == configuration.ReplierType {
		sourceController, err := controller.NewReplier(service.logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewReplier: %w", err)
		}
		source = sourceController
	} else if controllerType == configuration.PusherType {
		sourceController, err := controller.NewPull(service.logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewPull: %w", err)
		}
		source = sourceController
	} else {
		return fmt.Errorf("the '%s' controller type not supported", controllerType)
	}

	err := service.SetCustomSource(source)
	if err != nil {
		return fmt.Errorf("failed to add source controller: %w", err)
	}

	return nil
}

// SetCustomSource sets the source controller, and invokes the source controller's
func (service *Service) SetCustomSource(source controller.Interface) error {
	// todo move the below code to the two parts
	// move it to the validate, and to the Run()
	//controllerConf, err := service.configuration.Service[0].GetController(name)
	//if err != nil {
	//	return fmt.Errorf("the '%s' controller configuration wasn't found: %v", name, err)
	//}
	//source.AddConfig(controllerConf)
	service.source = source

	return nil
}

// Run the independent service.
func (service *Service) Run() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		err := service.source.Run()
		wg.Done()
		if err != nil {
			log.Fatal("failed to run the controller", "error", err)
		}
	}()

	// Run the proxy controller. Service controller itself on the other hand
	// will run the destination clients
	wg.Add(1)
	go func() {
		service.Controller.Run()
		wg.Done()
	}()

	println("waiting for the wait group")
	wg.Wait()
}
