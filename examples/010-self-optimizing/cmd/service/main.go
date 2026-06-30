package main

import (
	"fmt"

	"github.com/noPerfection/service"
)

const (
	serviceName            = "hello-world"
	serviceUrl             = "tmp/hello-world"
	serviceManagerUrl      = "tmp/hello-world_manager"
	defaultProxyName       = "default-name-proxy"
	defaultProxyUrl        = "tmp/default-name-proxy"
	defaultProxyManagerUrl = "tmp/default-name-proxy_manager"
	entrypointName         = "entrypoint"
	entrypointUrl          = "tmp/entrypoint"
	entrypointManagerUrl   = "tmp/entrypoint_manager" // since we treat it as inproc
	proxyModuleUrl         = "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#services/proxy?root=/home/medet/noPerfection/service/examples/010-self-optimizing"
	entrypointModuleUrl    = "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#services/entrypoint?root=/home/medet/noPerfection/service/examples/010-self-optimizing"
)

func main() {
	aiService, err := service.NewAiService()
	if err != nil {
		panic(err)
	}
	if err := aiService.Start(); err != nil {
		panic(err)
	}

	app, err := service.New(serviceName)
	if err != nil {
		panic(err)
	}

	app.SetServiceConfig(
		service.Config{
			Type:         service.IndependentType,
			Name:         serviceName,
			ModuleUrl:    "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#cmd/service?root=/home/medet/noPerfection/service/examples/010-self-optimizing",
			StartCommand: "./bin/service",
			Handlers: []service.Handler{
				service.IndependentHandler{
					Type:     service.SyncReplierType,
					Category: "main",
					Endpoint: service.Endpoint(serviceUrl, 0),
				},
				service.IndependentHandler{
					Type:     service.SyncReplierType,
					Category: service.ServiceManagerCategory,
					Endpoint: service.Endpoint(serviceManagerUrl, 0),
				},
			},
		},
	)

	if err := app.SetServiceConfig(service.Config{
		Type:         service.ProxyType,
		Name:         defaultProxyName,
		ModuleUrl:    proxyModuleUrl,
		StartCommand: "./bin/proxy",
		Handlers: []service.Handler{
			service.ProxyHandler{
				IndependentHandler: service.IndependentHandler{
					Type:     service.SyncReplierType,
					Category: "main",
					Endpoint: service.Endpoint(defaultProxyUrl, 0),
				},
				Routes: []string{"hello"},
			},
			service.IndependentHandler{
				Type:     service.SyncReplierType,
				Category: service.ServiceManagerCategory,
				Endpoint: service.Endpoint(defaultProxyManagerUrl, 0),
			},
		},
	}, "*pkg:$?var=services[name:"+defaultProxyName+"]"); err != nil {
		panic(err)
	}

	if err := app.SetServiceConfig(service.Config{
		Type:         service.ProxyType,
		Name:         entrypointName,
		ModuleUrl:    entrypointModuleUrl,
		StartCommand: "./bin/entrypoint",
		Handlers: []service.Handler{
			service.ProxyHandler{
				IndependentHandler: service.IndependentHandler{
					Type:     service.SyncReplierType,
					Category: "main",
					Endpoint: service.Endpoint(entrypointUrl, 0),
				},
				Routes: []string{service.AnyCmd},
			},
			service.IndependentHandler{
				Type:     service.SyncReplierType,
				Category: service.ServiceManagerCategory,
				Endpoint: service.Endpoint(entrypointManagerUrl, 0),
			},
		},
	}, "*pkg:$?var=services[name:"+entrypointName+"]"); err != nil {
		panic(err)
	}

	if err := app.SetHandlerDeps(service.Dependency{
		Name: service.DefaultHandlerCategory,
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", entrypointName),
			fmt.Sprintf("pkg:$?var=services[name:%s]", defaultProxyName),
		},
	}); err != nil {
		panic(err)
	}

	if err := app.SetHandlerDeps(service.Dependency{
		Name:       service.ServiceManagerCategory,
		Extensions: []string{service.AiServiceName},
	}); err != nil {
		panic(err)
	}

	app.Route("hello", onHello)
	app.Route("age-verification", onAgeVerification)

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("Started and ready!")

	app.Wait()
}

func onHello(req service.RequestInterface) service.ReplyInterface {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		return req.Fail("name is required")
	}

	return req.Ok(map[string]any{"message": "hello " + name})
}

func onAgeVerification(req service.RequestInterface) service.ReplyInterface {
	age, err := req.RouteParameters().Uint64Value("age")
	if err != nil {
		return req.Fail("age is required")
	}

	return req.Ok(map[string]any{"passed": age >= 18})
}
