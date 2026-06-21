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
	configPath    = "noPerfection.json"
	serviceName   = "hello-world"
	proxyName     = "default-name-proxy"
	proxyCategory = "main"
)

func main() {
	app, err := service.New(serviceName, configPath)
	if err != nil {
		panic(err)
	}

	if err := app.SetServiceConfig(defaultNameProxyConfig()); err != nil {
		panic(err)
	}
	if err := app.SetHandlerConfig(proxyManagerConfig(), proxyName); err != nil {
		panic(err)
	}
	if err := app.SetCommandDeps(topologyConfig.DepService{
		Name: "hello",
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", proxyName),
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
	fmt.Println("default-name-proxy dependency configured by command deps")

	app.Wait()
}

func defaultNameProxyConfig() topologyConfig.Service {
	return topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      proxyName,
		ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy/cmd/proxy",
		Handlers: []topologyConfig.Handler{
			topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: proxyCategory,
					Endpoint: message.NewEndpoint("localhost", 8001),
				},
				Routes: []string{"hello"},
			},
		},
	}
}

func proxyManagerConfig() topologyConfig.Handler {
	return topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: message.NewEndpoint("localhost", 8002),
	}
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		return req.Fail("name is required")
	}

	return req.Ok(datatype.New().Set("message", "hello "+name))
}
