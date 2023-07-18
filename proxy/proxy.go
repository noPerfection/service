// Package proxy defines the script that acts as the middleware
package proxy

// For proxy, there is no controllers.
// But only two kind of functions.
// The proxy enables the request and reply handlers.
//
//
/*Package independent is used to scaffold the independent service
 */

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"sync"
)

type Proxy struct {
	configuration configuration.Service
	sources       key_value.KeyValue
	controller    *Controller
}

const SourceName = "source"
const DestinationName = "destination"

// extension creates the parameters of the proxy controller.
// The proxy controller itself is added as the extension to the source controllers
func extension() *configuration.Extension {
	return &configuration.Extension{
		Name: ControllerName,
		Port: 0,
	}
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

		for _, instance := range c.Instances {
			err := proxyController.RegisterDestination(&instance)
			if err != nil {
				return fmt.Errorf("proxyController.RegistartionDestination: %w", err)
			}
		}
	}

	return nil
}

// New Proxy service based on the configurations
func New(serviceConf configuration.Service, logger log.Logger) (*Proxy, error) {
	if serviceConf.Type != configuration.ProxyType {
		return nil, fmt.Errorf("service type in the configuration is not Independent. It's '%s'", serviceConf.Type)
	}
	if err := validateConfiguration(serviceConf); err != nil {
		return nil, fmt.Errorf("validateConfiguration: %w", err)
	}

	proxyController, err := newController(logger)
	if err != nil {
		return nil, fmt.Errorf("newController: %w", err)
	}
	err = registerNonSources(serviceConf.Controllers, proxyController)
	if err != nil {
		return nil, fmt.Errorf("registerNonSources: %w", err)
	}

	service := Proxy{
		configuration: serviceConf,
		sources:       key_value.Empty(),
		controller:    proxyController,
	}

	return &service, nil
}

// SetRequestHandler sets the handler for all incoming requestMessages
func (service *Proxy) SetRequestHandler(handler RequestHandler) {
	service.controller.SetRequestHandler(handler)
}

func (service *Proxy) SetReplyHandler(handler ReplyHandler) {
	service.controller.SetReplyHandler(handler)
}

// AddSourceController sets the source controller, and invokes the source controller's
func (service *Proxy) AddSourceController(name string, source controller.Interface) error {
	controllerConf, err := service.configuration.GetController(name)
	if err != nil {
		return fmt.Errorf("the '%s' controller configuration wasn't found: %v", name, err)
	}
	source.AddConfig(controllerConf)
	service.sources.Set(name, source)

	return nil
}

// Run the independent service.
func (service *Proxy) Run() {
	var wg sync.WaitGroup

	proxyExtension := extension()

	// Run the sources
	for _, c := range service.configuration.Controllers {
		if err := service.sources.Exist(c.Name); err != nil {
			fmt.Println("the source is not included", c, "error", err)
			continue
		}
		controllerList := service.sources.Map()
		var c, ok = controllerList[c.Name].(controller.Interface)
		if !ok {
			fmt.Println("interface -> key-value", c)
			continue
		}

		// add the extensions required by the source controller
		requiredExtensions := c.RequiredExtensions()
		for _, name := range requiredExtensions {
			extension, err := service.configuration.GetExtension(name)
			if err != nil {
				log.Fatal("extension required by the controller doesn't exist in the configuration", "error", err)
			}

			c.AddExtensionConfig(extension)
		}

		// The proxy adds itself as the extension to the sources
		c.RequireExtension(proxyExtension.Name)
		c.AddExtensionConfig(proxyExtension)

		wg.Add(1)
		go func() {
			err := c.Run()
			wg.Done()
			if err != nil {
				log.Fatal("failed to run the controller", "error", err)
			}
		}()
	}

	// Run the proxy controller. Proxy controller itself on the other hand
	// will run the destination clients
	wg.Add(1)
	go func() {
		service.controller.Run()
		wg.Done()
	}()

	println("waiting for the wait group")
	wg.Wait()
}
