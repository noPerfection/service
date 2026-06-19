package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
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

	if err := app.Route("hello", onHello); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:8000")
	fmt.Println("default-name-proxy configured on localhost:8001")

	app.Wait()
}

func defaultNameProxyConfig() topologyConfig.Service {
	// return topologyConfig.Service{
	// 	Type:      topologyConfig.ProxyType,
	// 	Name:      proxyName,
	// 	ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy/cmd/proxy",
	// 	Handlers: []topologyConfig.HandlerVariant{
	// 		topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
	// 			Handler: topologyConfig.Handler{
	// 				Type:     topologyConfig.SyncReplierType,
	// 				Category: proxyCategory,
	// 				Endpoint: message.NewEndpoint("localhost", 8001),
	// 			},
	// 			Routes: []string{base.Any},
	// 			Outbounds: []topologyConfig.ServicePointer{
	// 				topologyConfig.ServiceTarget(topologyConfig.Service{
	// 					Type:      topologyConfig.IndependentType,
	// 					Name:      serviceName,
	// 					ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy/cmd/service",
	// 					Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
	// 						Type:     topologyConfig.ReplierType,
	// 						Category: handlers.DefaultHandlerCategory,
	// 						Endpoint: message.NewEndpoint("localhost", 8000),
	// 					}),
	// 				}),
	// 			},
	// 		}),
	// 	},
	// }
	return topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      proxyName,
		ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy/cmd/proxy",
		Handlers: []topologyConfig.HandlerVariant{
			topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
				Handler: topologyConfig.Handler{
					Type:     topologyConfig.SyncReplierType,
					Category: proxyCategory,
					Endpoint: message.NewEndpoint("localhost", 8001),
				},
				Routes: []string{base.Any},
				Outbounds: []string{
					fmt.Sprintf("pkg:$?var=services[name:%s]&category=main", serviceName),
				},
			}),
		},
	}
}

// func defaultNameProxyConfig2() topologyConfig.Service {
// 	return topologyConfig.Service{
// 		Type: topologyConfig.ProxyType,
// 		Name: proxyName,
// 	}
// }

// func defaultProxyManagerConfig2() topologyConfig.HandlerVariant {
// 	return topologyConfig.NewHandlerVariant(topologyConfig.Handler{
// 		Type:     topologyConfig.SyncReplierType,
// 		Category: topology.ServiceManagerCategory,
// 		Endpoint: message.NewEndpoint("localhost", 8002),
// 	})
// }

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		return req.Fail("name is required")
	}

	return req.Ok(datatype.New().Set("message", "hello "+name))
}
