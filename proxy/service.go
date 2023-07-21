// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
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
	logger     *log.Logger
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

// registerDestination registers the controller instances as the destination.
func (service *Service) registerDestination() {
	for _, c := range service.configuration.Service.Controllers {
		if c.Name == DestinationName {
			service.Controller.RegisterDestination(&c)
			break
		}
	}
}

// registerSource adds the configuration to the source.
func (service *Service) registerSource() {
	for _, c := range service.configuration.Service.Controllers {
		if c.Name == SourceName {
			service.source.AddConfig(&c)
			break
		}
	}
}

// New proxy service along with its controller.
func New(config *configuration.Config, parent *log.Logger) *Service {
	logger := parent.Child("proxy")

	service := Service{
		configuration: config,
		source:        nil,
		Controller:    newController(logger.Child("controller")),
		logger:        logger,
	}

	return &service
}

// prepareConfiguration creates a configuration.
// If the configuration was already given, then it validates it.
func (service *Service) prepareConfiguration() error {
	// validate the service itself
	config := service.configuration
	serviceConfig := service.configuration.Service
	if len(serviceConfig.Type) == 0 {
		serviceConfig = configuration.Service{
			Type:     configuration.ProxyType,
			Name:     config.Name + "proxy",
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != configuration.ProxyType {
		return fmt.Errorf("service type is overwritten. It's not proxy its '%s'", serviceConfig.Type)
	}

	// validate the controllers
	// it means it should have two controllers: source and destination
	var sourceConfig configuration.Controller
	var destinationConfig configuration.Controller
	for _, c := range serviceConfig.Controllers {
		if c.Name == SourceName {
			sourceConfig = c
		} else if c.Name == DestinationName {
			destinationConfig = c
		}
	}

	if len(sourceConfig.Type) == 0 {
		sourceConfig = configuration.Controller{
			Type: service.source.ControllerType(),
			Name: SourceName,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, sourceConfig)
	} else {
		service.logger.Info("the source config is", sourceConfig)
		if sourceConfig.Type != service.source.ControllerType() {
			return fmt.Errorf("source expected to be of %s type, but in the config it's %s of type",
				service.source.ControllerType(), sourceConfig.Type)
		}
	}

	if len(destinationConfig.Type) == 0 {
		destinationConfig = configuration.Controller{
			Type: service.Controller.requiredDestination,
			Name: DestinationName,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, destinationConfig)
	} else {
		if destinationConfig.Type != service.Controller.requiredDestination {
			return fmt.Errorf("destination expected to be of %s type, but in the config it's %s of type",
				service.Controller.requiredDestination, destinationConfig.Type)
		}
	}

	// validate the controller instances
	// make sure that they are tpc type
	if len(sourceConfig.Instances) == 0 {
		port := service.configuration.GetFreePort()

		sourceInstance := configuration.ControllerInstance{
			Name:     sourceConfig.Name,
			Instance: sourceConfig.Name + "1",
			Port:     uint64(port),
		}
		sourceConfig.Instances = append(sourceConfig.Instances, sourceInstance)
	} else {
		if sourceConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	if len(destinationConfig.Instances) == 0 {
		port := service.configuration.GetFreePort()

		sourceInstance := configuration.ControllerInstance{
			Name:     destinationConfig.Name,
			Instance: destinationConfig.Name + "1",
			Port:     uint64(port),
		}
		destinationConfig.Instances = append(destinationConfig.Instances, sourceInstance)
	} else {
		if destinationConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	service.configuration.Service = serviceConfig

	// todo validate the extensions
	// todo validate the proxies
	return nil
}

func (service *Service) Prepare() error {
	if service.source == nil {
		return fmt.Errorf("missing source. call service.SetDefaultSource")
	}

	if service.Controller.requiredDestination == configuration.UnknownType {
		return fmt.Errorf("missing the required destination. call service.Controller.RequireDestination")
	}

	err := service.prepareConfiguration()
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	service.registerDestination()
	service.registerSource()

	proxyExtension := extension()

	// Run the sources
	// add the extensions required by the source controller
	//requiredExtensions := service.source.RequiredExtensions()
	//for _, name := range requiredExtensions {
	//	extension, err := service.configuration.Service.GetExtension(name)
	//	if err != nil {
	//		log.Fatal("extension required by the controller doesn't exist in the configuration", "error", err)
	//	}
	//
	//	service.source.AddExtensionConfig(extension)
	//}

	// The proxy adds itself as the extension to the sources
	// after validation of the previous extensions
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
	service.source = source

	return nil
}

func (service *Service) generateConfiguration() {
	path, err := argument.Value(argument.Path)
	if err != nil {
		service.logger.Fatal("generate configuration requires a path flag", "error", err)
	}

	err = service.configuration.WriteService(path)
	if err != nil {
		service.logger.Fatal("failed to write the service into the file", "error", err)
	}

	service.logger.Info("the service was generated", "path", path)
}

// Run the proxy service.
func (service *Service) Run() {
	if argument.Exist(argument.GenerateConfiguration) {
		service.generateConfiguration()
		return
	}

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
