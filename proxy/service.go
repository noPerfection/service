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

// Proxy defines the parameters of the proxy service
type Proxy struct {
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
// It adds the controller configuration.
func (proxy *Proxy) registerDestination() {
	for _, c := range proxy.configuration.Service.Controllers {
		if c.Name == DestinationName {
			proxy.Controller.RegisterDestination(&c)
			break
		}
	}
}

// registerSource adds the configuration to the source.
func (proxy *Proxy) registerSource() {
	for _, c := range proxy.configuration.Service.Controllers {
		if c.Name == SourceName {
			proxy.source.AddConfig(&c)
			break
		}
	}
}

// New proxy service along with its controller.
func New(config *configuration.Config, parent *log.Logger) *Proxy {
	logger := parent.Child("proxy")

	service := Proxy{
		configuration: config,
		source:        nil,
		Controller:    newController(logger.Child("controller")),
		logger:        logger,
	}

	return &service
}

// prepareConfiguration creates a configuration.
// If the configuration was already given, then it validates it.
func (proxy *Proxy) prepareConfiguration() error {
	// validate the proxy itself
	config := proxy.configuration
	serviceConfig := proxy.configuration.Service
	if len(serviceConfig.Type) == 0 {
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			proxy.logger.Fatal("failed to get os context", "error", err)
		}

		serviceConfig = configuration.Service{
			Type:     configuration.ProxyType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != configuration.ProxyType {
		return fmt.Errorf("proxy type is overwritten. It's not proxy its '%s'", serviceConfig.Type)
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
			Type: proxy.source.ControllerType(),
			Name: SourceName,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, sourceConfig)
	} else {
		if sourceConfig.Type != proxy.source.ControllerType() {
			return fmt.Errorf("source expected to be of %s type, but in the config it's %s of type",
				proxy.source.ControllerType(), sourceConfig.Type)
		}
	}

	if len(destinationConfig.Type) == 0 {
		destinationConfig = configuration.Controller{
			Type: proxy.Controller.requiredDestination,
			Name: DestinationName,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, destinationConfig)
	} else {
		if destinationConfig.Type != proxy.Controller.requiredDestination {
			return fmt.Errorf("destination expected to be of %s type, but in the config it's %s of type",
				proxy.Controller.requiredDestination, destinationConfig.Type)
		}
	}

	// validate the controller instances
	// make sure that they are tpc type
	if len(sourceConfig.Instances) == 0 {
		port := proxy.configuration.GetFreePort()

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
		port := proxy.configuration.GetFreePort()

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

	serviceConfig.SetController(sourceConfig)
	serviceConfig.SetController(destinationConfig)
	proxy.configuration.Service = serviceConfig

	// todo validate the extensions
	// todo validate the proxies
	return nil
}

// ServiceToProxy returns the service in the proxy format
// so that it can be used as a proxy
func ServiceToProxy(s *configuration.Service) (configuration.Proxy, error) {
	if s.Type != configuration.ProxyType {
		return configuration.Proxy{}, fmt.Errorf("only proxy type of service can be converted")
	}

	controller, err := s.GetController(SourceName)
	if err != nil {
		return configuration.Proxy{}, fmt.Errorf("no source controller: %w", err)
	}

	if len(controller.Instances) == 0 {
		return configuration.Proxy{}, fmt.Errorf("no source instances")
	}

	converted := configuration.Proxy{
		Url:      s.Url,
		Instance: controller.Name + " instance 01",
		Port:     controller.Instances[0].Port,
	}

	return converted, nil
}

func (proxy *Proxy) Prepare() error {
	if proxy.source == nil {
		return fmt.Errorf("missing source. call proxy.SetDefaultSource")
	}

	if proxy.Controller.requiredDestination == configuration.UnknownType {
		return fmt.Errorf("missing the required destination. call proxy.Controller.RequireDestination")
	}

	err := proxy.prepareConfiguration()
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	proxy.registerDestination()
	proxy.registerSource()

	proxyExtension := extension()

	// Run the sources
	// add the extensions required by the source controller
	//requiredExtensions := proxy.source.RequiredExtensions()
	//for _, name := range requiredExtensions {
	//	extension, err := proxy.configuration.Proxy.GetExtension(name)
	//	if err != nil {
	//		log.Fatal("extension required by the controller doesn't exist in the configuration", "error", err)
	//	}
	//
	//	proxy.source.AddExtensionConfig(extension)
	//}

	// The proxy adds itself as the extension to the sources
	// after validation of the previous extensions
	proxy.source.RequireExtension(proxyExtension.Url)
	proxy.source.AddExtensionConfig(proxyExtension)

	return nil
}

// SetDefaultSource creates a source controller of the given type.
//
// It loads the source name automatically.
func (proxy *Proxy) SetDefaultSource(controllerType configuration.Type) error {
	// todo move the validation to the proxy.ValidateTypes() function
	var source controller.Interface
	if controllerType == configuration.ReplierType {
		sourceController, err := controller.NewReplier(proxy.logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewReplier: %w", err)
		}
		source = sourceController
	} else if controllerType == configuration.PusherType {
		sourceController, err := controller.NewPull(proxy.logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewPull: %w", err)
		}
		source = sourceController
	} else {
		return fmt.Errorf("the '%s' controller type not supported", controllerType)
	}

	err := proxy.SetCustomSource(source)
	if err != nil {
		return fmt.Errorf("failed to add source controller: %w", err)
	}

	return nil
}

// SetCustomSource sets the source controller, and invokes the source controller's
func (proxy *Proxy) SetCustomSource(source controller.Interface) error {
	proxy.source = source

	return nil
}

func (proxy *Proxy) generateConfiguration() {
	path, err := argument.Value(argument.Path)
	if err != nil {
		proxy.logger.Fatal("requires 'path' flag", "error", err)
	}

	url, err := argument.Value(argument.Url)
	if err != nil {
		proxy.logger.Fatal("requires 'url' flag", "error", err)
	}

	proxy.configuration.Service.Url = url

	err = proxy.configuration.WriteService(path)
	if err != nil {
		proxy.logger.Fatal("failed to write the proxy into the file", "error", err)
	}

	proxy.logger.Info("the proxy was generated", "path", path)
}

// Run the proxy service.
func (proxy *Proxy) Run() {
	if argument.Exist(argument.BuildConfiguration) {
		proxy.generateConfiguration()
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		err := proxy.source.Run()
		wg.Done()
		if err != nil {
			log.Fatal("failed to run the controller", "error", err)
		}
	}()

	// Run the proxy controller. Proxy controller itself on the other hand
	// will run the destination clients
	wg.Add(1)
	go func() {
		proxy.Controller.Run()
		wg.Done()
	}()

	wg.Wait()
}
