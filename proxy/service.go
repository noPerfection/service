// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	service2 "github.com/ahmetson/service-lib/configuration/service"
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

// extension creates the configuration of the proxy controller.
// The proxy controller itself is added as the extension to the source controllers,
// to the request handlers and to the reply handlers.
func extension() *service2.Extension {
	return service2.NewInternalExtension(ControllerName)
}

// registerDestination registers the controller instances as the destination.
// It adds the controller configuration.
func (proxy *Proxy) registerDestination() {
	for _, c := range proxy.service.Config.Service.Controllers {
		if c.Category == service2.DestinationName {
			proxy.Controller.RegisterDestination(c, proxy.service.Config.Service.Url)
			break
		}
	}
}

// New proxy service along with its controller.
func New(config *configuration.Config, parent *log.Logger) *Proxy {
	logger := parent.Child("service", "service_type", service2.ProxyType)

	base, _ := independent.New(config, logger)

	service := Proxy{
		service:    base,
		Controller: newController(logger.Child("controller")),
	}

	return &service
}

func (proxy *Proxy) getSource() controller.Interface {
	controllers := proxy.service.Controllers.Map()
	source := controllers[service2.SourceName].(controller.Interface)
	return source
}

func (proxy *Proxy) Prepare() error {
	if proxy.Controller.requiredDestination == service2.UnknownType {
		return fmt.Errorf("missing the required destination. call proxy.ControllerCategory.RequireDestination")
	}

	if err := proxy.service.Prepare(service2.ProxyType); err != nil {
		return fmt.Errorf("service.Prepare as '%s' failed: %w", service2.ProxyType, err)
	}

	if err := proxy.service.PrepareControllerConfiguration(service2.DestinationName, proxy.Controller.requiredDestination); err != nil {
		return fmt.Errorf("prepare destination as '%s' failed: %w", proxy.Controller.requiredDestination, err)
	}

	proxy.registerDestination()

	return nil
}

// SetDefaultSource creates a source controller of the given type.
//
// It loads the source name automatically.
func (proxy *Proxy) SetDefaultSource(controllerType service2.ControllerType) error {
	// todo move the validation to the proxy.ValidateTypes() function
	var source controller.Interface
	if controllerType == service2.SyncReplierType {
		sourceController, err := controller.SyncReplier(proxy.service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as controller.NewReplier: %w", err)
		}
		source = sourceController
	} else if controllerType == service2.PusherType {
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
	proxy.service.AddController(service2.SourceName, source)
}

// Run the proxy service.
func (proxy *Proxy) Run() {
	// call BuildConfiguration explicitly to generate the yaml without proxy controller's extension.
	proxy.service.BuildConfiguration()

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
