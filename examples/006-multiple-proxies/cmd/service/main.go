package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	configPath          = "noPerfection.json"
	serviceName         = "hello-world"
	defaultProxyName    = "default-name-proxy"
	formatProxyName     = "upper-case-names"
	proxyCategory       = "main"
	defaultProxyPort    = 8001
	defaultManagerPort  = 8002
	formatProxyPort     = 8003
	formatManagerPort   = 8004
	defaultProxyPackage = "github.com/noPerfection/service/examples/006-multiple-proxies/cmd/proxy"
	formatProxyPackage  = "github.com/noPerfection/service/examples/006-multiple-proxies/cmd/proxy2"
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
	if err := app.SetServiceConfig(proxyConfig(formatProxyName, formatProxyPackage, formatProxyPort)); err != nil {
		panic(err)
	}
	if err := app.SetHandlerConfig(proxyManagerConfig(formatManagerPort), formatProxyName); err != nil {
		panic(err)
	}
	if err := app.SetCommandDeps(topologyConfig.DepService{
		Name: "hello",
		Proxies: []topologyConfig.ServicePointer{
			topologyConfig.RefTarget(defaultProxyName),
			topologyConfig.RefTarget(formatProxyName),
		},
	}); err != nil {
		panic(err)
	}

	if err := app.Route("hello", onHello); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:8000")
	fmt.Println("command deps chain: default-name-proxy -> upper-case-names -> hello-world")

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
