package main

import (
	"fmt"
	"os"

	"github.com/noPerfection/os/env"
	"github.com/noPerfection/service"
	// "github.com/noPerfection/service/inproc_topology"
)

const (
	serviceName      = "hello-world"
	defaultProxyName = "default-name-proxy"
	entrypointName   = "entrypoint"
)

func main() {
	env.LoadAnyEnv()
	aiService, err := service.NewAiService(os.Getenv("ANTHROPIC_API_KEY"))
	if err != nil {
		panic(err)
	}
	if err := aiService.CheckConnection(); err != nil {
		panic(err)
	} else {
		fmt.Println("AI connection successful")
	}

	// topology := &inproc_topology.InprocTopology{}
	// if err := topology.Start(); err != nil {
	// 	panic(err)
	// }

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
		ModuleUrl:    "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#cmd/proxy",
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
		ModuleUrl:    "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#cmd/entrypoint",
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
