package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	configPath          = "noPerfection.json"
	serviceName         = "hello-world"
	defaultProxyName    = "default-name-proxy"
	entrypointName      = "entrypoint"
	proxyCategory       = "main"
	defaultProxyPort    = 8001
	defaultManagerPort  = 8002
	entrypointPort      = 8003
	entrypointManager   = 8004
	defaultProxyPackage = "github.com/noPerfection/service/examples/007-handler-deps/cmd/proxy"
	entrypointPackage   = "github.com/noPerfection/service/examples/007-handler-deps/cmd/entrypoint"
)

func main() {
	app, err := service.New(serviceName, configPath)
	if err != nil {
		panic(err)
	}

	if err := app.SetServiceConfig(proxyConfig(defaultProxyName, defaultProxyPackage, defaultProxyPort)); err != nil {
		panic(err)
	}
	if err := app.SetHandlerConfig(proxyManagerConfig(defaultManagerPort), defaultProxyName); err != nil {
		panic(err)
	}
	if err := app.SetServiceConfig(proxyConfig(entrypointName, entrypointPackage, entrypointPort)); err != nil {
		panic(err)
	}
	if err := app.SetHandlerConfig(proxyManagerConfig(entrypointManager), entrypointName); err != nil {
		panic(err)
	}
	if err := app.SetHandlerDeps(topologyConfig.DepService{
		Name: handlers.DefaultHandlerCategory,
		Proxies: []topologyConfig.ServicePointer{
			topologyConfig.RefTarget(entrypointName),
		},
	}); err != nil {
		panic(err)
	}
	if err := app.SetCommandDeps(topologyConfig.DepService{
		Name: "hello",
		Proxies: []topologyConfig.ServicePointer{
			topologyConfig.RefTarget(defaultProxyName),
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
	fmt.Println("entrypoint proxy exposes hello and age-verification on localhost:8003")

	app.Wait()
}

func proxyConfig(name string, moduleURL string, port uint64) topologyConfig.Service {
	return topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      name,
		ModuleUrl: moduleURL,
		Handlers: []topologyConfig.HandlerVariant{
			topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
				Handler: topologyConfig.Handler{
					Type:     topologyConfig.SyncReplierType,
					Category: proxyCategory,
					Endpoint: message.NewEndpoint("localhost", port),
				},
			}),
		},
	}
}

func proxyManagerConfig(port uint64) topologyConfig.Handler {
	return topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: message.NewEndpoint("localhost", port),
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
