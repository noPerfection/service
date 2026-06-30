package main

import (
	"fmt"
	"runtime/debug"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	configPath           = "noPerfection.json"
	serviceName          = "hello-world"
	defaultProxyName     = "default-name-proxy"
	entrypointName       = "entrypoint"
	proxyCategory        = "main"
	serviceManagerPort   = 8001
	defaultProxyEndpoint = "tmp/default_name_proxy"
	defaultProxyManager  = "tmp/default_name_proxy_manager"
	entrypointEndpoint   = "tmp/entrypoint_proxy"
	entrypointManager    = "tmp/entrypoint_proxy_manager"
	defaultProxyPackage  = "github.com/noPerfection/service/examples/008-autostart-deps/cmd/proxy"
	entrypointPackage    = "github.com/noPerfection/service/examples/008-autostart-deps/cmd/entrypoint"
	defaultProxyStartCommand = "./bin/proxy"
	entrypointStartCommand   = "./bin/entrypoint"
)

func main() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		// binary built without modules, or very old toolchain
	}

	moduleURL := info.Main.Path
	moduleVersion := info.Main.Version
	mainPackage := info.Path
	fmt.Println("Module URL:", moduleURL, "Module Version:", moduleVersion, "Main Package:", mainPackage)

	app, err := service.New(serviceName)
	if err != nil {
		panic(err)
	}

	if err := app.SetEndpoint(message.NewEndpoint("localhost", serviceManagerPort), topology.ServiceManagerCategory); err != nil {
		panic(err)
	}

	defaultProxy := proxyConfig(defaultProxyName, defaultProxyPackage, defaultProxyEndpoint, defaultProxyStartCommand, defaultProxyManager)
	if err := app.SetServiceConfig(defaultProxy); err != nil {
		panic(err)
	}

	entrypoint := proxyConfig(entrypointName, entrypointPackage, entrypointEndpoint, entrypointStartCommand, entrypointManager)
	if err := app.SetServiceConfig(entrypoint); err != nil {
		panic(err)
	}
	if err := app.SetHandlerDeps(topologyConfig.DepService{
		Name: handlers.DefaultHandlerCategory,
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", entrypointName),
		},
	}); err != nil {
		panic(err)
	}
	if err := app.SetCommandDeps(topologyConfig.DepService{
		Name: "hello",
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", defaultProxyName),
		},
	}); err != nil {
		panic(err)
	}

	if err := app.Route("hello", onHello); err != nil {
		panic(err)
	}
	if err := app.Route("age-verification", onAgeVerification); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:8000")
	fmt.Println("entrypoint proxy exposes hello and age-verification on", entrypointEndpoint)
	fmt.Println("dependent proxies are started automatically")

	app.Wait()
}

func proxyConfig(name string, moduleURL string, endpointID string, startCommand string, managerEndpointID string) topologyConfig.Service {
	return topologyConfig.Service{
		Type:         topologyConfig.ProxyType,
		Name:         name,
		ModuleUrl:    moduleURL,
		StartCommand: startCommand,
		Handlers: []topologyConfig.Handler{
			topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: proxyCategory,
					Endpoint: message.NewEndpoint(endpointID, 0),
				},
				Routes: []string{"hello", "age-verification"},
			},
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topology.ServiceManagerCategory,
				Endpoint: message.NewEndpoint(managerEndpointID, 0),
			},
		},
	}
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		return req.Fail("name is required")
	}

	return req.Ok(datatype.New().Set("message", "hello "+name))
}

func onAgeVerification(req message.RequestInterface) message.ReplyInterface {
	age, err := req.RouteParameters().Uint64Value("age")
	if err != nil {
		return req.Fail("age is required")
	}

	return req.Ok(datatype.New().Set("passed", age >= 18))
}
