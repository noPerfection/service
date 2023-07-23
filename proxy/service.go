// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/independent"
	"github.com/ahmetson/service-lib/log"
	"sync"
)

type service = independent.Service

// Proxy defines the parameters of the proxy service
type Proxy struct {
	*service
	// Controller that handles the requests and redirects to the destination.
	Controller *Controller
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
	for _, c := range proxy.service.Config.Service.Controllers {
		if c.Name == DestinationName {
			proxy.Controller.RegisterDestination(&c)
			break
		}
	}
}

// New proxy service along with its controller.
func New(config *configuration.Config, parent *log.Logger) *Proxy {
	logger := parent.Child("proxy")

	service := Proxy{
		service: &independent.Service{
			Config:      config,
			Logger:      logger,
			Controllers: key_value.Empty(),
		},
		Controller: newController(logger.Child("controller")),
	}

	return &service
}

func (proxy *Proxy) getSource() controller.Interface {
	controllers := proxy.service.Controllers.Map()
	source := controllers[SourceName].(controller.Interface)
	return source
}

// ServiceToProxy returns the service in the proxy format
// so that it can be used as a proxy
func ServiceToProxy(s *configuration.Service) (configuration.Proxy, error) {
	if s.Type != configuration.ProxyType {
		return configuration.Proxy{}, fmt.Errorf("only proxy type of service can be converted")
	}

	controllerConfig, err := s.GetController(SourceName)
	if err != nil {
		return configuration.Proxy{}, fmt.Errorf("no source controllerConfig: %w", err)
	}

	if len(controllerConfig.Instances) == 0 {
		return configuration.Proxy{}, fmt.Errorf("no source instances")
	}

	converted := configuration.Proxy{
		Url:      s.Url,
		Instance: controllerConfig.Name + " instance 01",
		Port:     controllerConfig.Instances[0].Port,
	}

	return converted, nil
}

func (proxy *Proxy) Prepare() error {
	if proxy.Controller.requiredDestination == configuration.UnknownType {
		return fmt.Errorf("missing the required destination. call proxy.Controller.RequireDestination")
	}

	if err := proxy.service.Prepare(configuration.ProxyType); err != nil {
		return fmt.Errorf("service.Prepare as '%s' failed: %w", configuration.ProxyType, err)
	}

	if err := proxy.service.PrepareControllerConfiguration(DestinationName, proxy.Controller.requiredDestination); err != nil {
		return fmt.Errorf("prepare destination as '%s' failed: %w", proxy.Controller.requiredDestination, err)
	}

	proxy.registerDestination()

	return nil
}

// SetDefaultSource creates a source controller of the given type.
//
// It loads the source name automatically.
func (proxy *Proxy) SetDefaultSource(controllerType configuration.Type) error {
	// todo move the validation to the proxy.ValidateTypes() function
	var source controller.Interface
	if controllerType == configuration.ReplierType {
		sourceController, err := controller.NewReplier(proxy.service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewReplier: %w", err)
		}
		source = sourceController
	} else if controllerType == configuration.PusherType {
		sourceController, err := controller.NewPull(proxy.service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewPull: %w", err)
		}
		source = sourceController
	} else {
		return fmt.Errorf("the '%s' controller type not supported", controllerType)
	}

	proxy.SetCustomSource(source)

	return nil
}

// SetCustomSource sets the source controller, and invokes the source controller's
func (proxy *Proxy) SetCustomSource(source controller.Interface) {
	proxy.service.AddController(SourceName, source)
}

// Run the proxy service.
func (proxy *Proxy) Run() {
	if argument.Exist(argument.BuildConfiguration) {
		proxy.service.BuildConfiguration()
	}

	// we add the proxy extension to the source.
	// source can forward messages along with a route.
	proxyExtension := extension()

	// The proxy adds itself as the extension to the sources
	// after validation of the previous extensions
	proxy.getSource().RequireExtension(proxyExtension.Url)
	proxy.getSource().AddExtensionConfig(proxyExtension)
	go proxy.service.Run()

	// Run the proxy controller. Proxy controller itself on the other hand
	// will run the destination clients
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		proxy.Controller.Run()
		wg.Done()
	}()

	wg.Wait()
}
