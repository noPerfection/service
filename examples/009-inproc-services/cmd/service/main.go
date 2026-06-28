package main

import (
	"fmt"

	"github.com/noPerfection/service"
)

const (
	serviceName         = "hello-world"
	defaultProxyName    = "default-name-proxy"
	entrypointName      = "entrypoint"
	proxyModuleUrl      = "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#services/proxy?root=/home/medet/noPerfection/service/examples/009-inproc-services"
	entrypointModuleUrl = "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#services/entrypoint?root=/home/medet/noPerfection/service/examples/009-inproc-services"
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

	app.SetHandlerConfig(
		service.IndependentHandler{
			Type:     service.SyncReplierType,
			Category: "main",
			Endpoint: service.Endpoint(serviceName, 0),
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
					Endpoint: service.Endpoint(defaultProxyName, 0),
				},
				Routes: []string{"hello"},
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
					Endpoint: service.Endpoint(entrypointName, 0),
				},
				Routes: []string{service.AnyCmd},
			},
		},
	}, "*pkg:$?var=services[name:"+entrypointName+"]"); err != nil {
		panic(err)
	}

	if err := app.SetHandlerDeps(service.Dependency{
		Name: service.DefaultHandlerCategory,
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", entrypointName),
		},
	}); err != nil {
		panic(err)
	}
	if err := app.SetCommandDeps(service.Dependency{
		Name: "hello",
		Proxies: []string{
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
	if err := startInprocTopology(); err != nil {
		panic(err)
	}

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
