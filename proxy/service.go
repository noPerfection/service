// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/config"
	service2 "github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/server"
	"github.com/ahmetson/service-lib/service"
	"sync"
)

type _service = service.Service

// Proxy defines the parameters of the proxy service
type Proxy struct {
	*_service
	// Controller that handles the requests and redirects to the destination.
	Controller *Controller
}

// An extension creates the config of the proxy server.
// The proxy server itself is added as the extension to the source controllers,
// to the request handlers and to the reply handlers.
func extension() *service2.Extension {
	return service2.NewInternalExtension(ControllerName)
}

// registerDestination registers the server instances as the destination.
// It adds the server config.
func (proxy *Proxy) registerDestination() {
	for _, c := range proxy._service.Config.Controllers {
		if c.Category == service2.DestinationName {
			proxy.Controller.RegisterDestination(c, proxy._service.Config.Url)
			break
		}
	}
}

// New proxy service along with its server.
func New(conf *config.Service, parent *log.Logger) *Proxy {
	logger := parent.Child("service", "service_type", config.ProxyType)

	base, _ := service.New(conf, logger)

	_service := Proxy{
		_service:   base,
		Controller: newController(logger.Child("server")),
	}

	return &_service
}

func (proxy *Proxy) getSource() server.Interface {
	controllers := proxy._service.Controllers.Map()
	source := controllers[service2.SourceName].(server.Interface)
	return source
}

func (proxy *Proxy) Prepare() error {
	if proxy.Controller.requiredDestination == service2.UnknownType {
		return fmt.Errorf("missing the required destination. call proxy.ControllerCategory.RequireDestination")
	}

	if err := proxy._service.Prepare(config.ProxyType); err != nil {
		return fmt.Errorf("service.Run as '%s' failed: %w", config.ProxyType, err)
	}

	if err := proxy._service.PrepareControllerConfiguration(service2.DestinationName, proxy.Controller.requiredDestination); err != nil {
		return fmt.Errorf("prepare destination as '%s' failed: %w", proxy.Controller.requiredDestination, err)
	}

	proxy.registerDestination()

	return nil
}

// SetDefaultSource creates a source server of the given type.
//
// It loads the source name automatically.
func (proxy *Proxy) SetDefaultSource(controllerType service2.ControllerType) error {
	// todo move the validation to the proxy.ValidateTypes() function
	var source server.Interface
	if controllerType == service2.SyncReplierType {
		sourceController, err := server.SyncReplier(proxy._service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as server.NewReplier: %w", err)
		}
		source = sourceController
	} else if controllerType == service2.PusherType {
		sourceController, err := server.NewPull(proxy._service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as server.NewPull: %w", err)
		}
		source = sourceController
	} else {
		return fmt.Errorf("the '%s' server type not supported", controllerType)
	}

	proxy.SetCustomSource(source)

	return nil
}

// SetCustomSource sets the source server, and invokes the source server's
func (proxy *Proxy) SetCustomSource(source server.Interface) {
	proxy._service.AddController(service2.SourceName, source)
}

// Run the proxy service.
func (proxy *Proxy) Run() {
	// call BuildConfiguration explicitly to generate the yaml without proxy server's extension.
	proxy._service.BuildConfiguration()

	// we add the proxy extension to the source.
	// source can forward messages along with a route.
	proxyExtension := extension()

	// The proxy adds itself as the extension to the sources
	// after validation of the previous extensions
	proxy.getSource().RequireExtension(proxyExtension.Url)
	proxy.getSource().AddExtensionConfig(proxyExtension)
	go proxy._service.Run()

	// Run the proxy server. Proxy server itself on the other hand
	// will run the destination clients
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		proxy.Controller.Run()
		wg.Done()
	}()

	wg.Wait()
}
