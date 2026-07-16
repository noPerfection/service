package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/manager"
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
	metricsName            = "metrics"
	metricsUrl             = "tmp/metrics"
	metricsManagerUrl      = "tmp/metrics_manager"
	proxy2Name             = "upper-case-names"
	proxy2Url              = "tmp/upper-case-names"
	proxy2ManagerUrl       = "tmp/upper-case-names_manager"
	exampleRoot            = "/home/medet/noPerfection/service/examples/011-security"
	proxyModuleUrl         = "pkg:golang/github.com/noPerfection/service/examples/011-security#services/proxy?root=" + exampleRoot
	entrypointModuleUrl    = "pkg:golang/github.com/noPerfection/service/examples/011-security#services/entrypoint?root=" + exampleRoot
	serviceModuleUrl       = "pkg:golang/github.com/noPerfection/service/examples/011-security#cmd/service?root=" + exampleRoot
	metricsModuleUrl       = "pkg:golang/github.com/noPerfection/service/examples/011-security#services/metrics?root=" + exampleRoot
	proxy2ModuleUrl        = "pkg:golang/github.com/noPerfection/service/examples/011-security#services/proxy2?root=" + exampleRoot
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
			ModuleUrl:    serviceModuleUrl,
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
			Parameters: service.KeyValue().Set("manager-secret-key",
				"*pkg:os/env?var=MANAGER_SECRET_KEY&LoadAnyEnv=true").Set("public-key",
				"*pkg:os/env?var=MANAGER_PUBLIC_KEY&LoadAnyEnv=true"),
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
				Routes: []string{message.Any},
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

	if err := app.SetServiceConfig(service.Config{
		Type:         service.ProxyType,
		Name:         metricsName,
		ModuleUrl:    metricsModuleUrl,
		StartCommand: "./bin/metrics",
		Handlers: []service.Handler{
			service.ProxyHandler{
				IndependentHandler: service.IndependentHandler{
					Type:     service.SyncReplierType,
					Category: "main",
					Endpoint: service.Endpoint(metricsUrl, 0),
				},
				Routes: []string{message.Any},
			},
			service.IndependentHandler{
				Type:     service.SyncReplierType,
				Category: service.ServiceManagerCategory,
				Endpoint: service.Endpoint(metricsManagerUrl, 0),
			},
		},
	}, "*pkg:$?var=services[name:"+metricsName+"]"); err != nil {
		panic(err)
	}

	if err := app.SetServiceConfig(service.Config{
		Type:         service.ProxyType,
		Name:         proxy2Name,
		ModuleUrl:    proxy2ModuleUrl,
		StartCommand: "./bin/proxy2",
		Handlers: []service.Handler{
			service.ProxyHandler{
				IndependentHandler: service.IndependentHandler{
					Type:     service.SyncReplierType,
					Category: "main",
					Endpoint: service.Endpoint(proxy2Url, 0),
				},
				Routes: []string{"hello"},
			},
			service.IndependentHandler{
				Type:     service.SyncReplierType,
				Category: service.ServiceManagerCategory,
				Endpoint: service.Endpoint(proxy2ManagerUrl, 0),
			},
		},
	}, "*pkg:$?var=services[name:"+proxy2Name+"]"); err != nil {
		panic(err)
	}

	if err := app.SetHandlerDeps(service.Dependency{
		Name: service.DefaultHandlerCategory,
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", entrypointName),
			fmt.Sprintf("pkg:$?var=services[name:%s]", metricsName),
		},
	}); err != nil {
		panic(err)
	}
	app.SetCommandDeps(service.Dependency{
		Name: "hello",
		Proxies: []string{
			fmt.Sprintf("pkg:$?var=services[name:%s]", defaultProxyName),
			fmt.Sprintf("pkg:$?var=services[name:%s]", proxy2Name),
		},
	})

	if err := app.SetHandlerDeps(service.Dependency{
		Name:       service.ServiceManagerCategory,
		Extensions: []string{service.AiServiceName},
	}); err != nil {
		panic(err)
	}

	onHello := func(req service.RequestInterface) service.ReplyInterface {
		name, err := req.RouteParameters().StringValue("name")
		if err != nil || name == "" {
			return req.Fail("name is required")
		}

		aiMgrClient, err := client.New(service.DefaultAiManagerEndpoint.Id, service.DefaultAiManagerEndpoint.Port, client.ReplierType)
		if err != nil {
			return req.Fail("ai manager client: " + err.Error())
		}
		defer aiMgrClient.Close()
		aiMgrClient.Timeout(time.Second * 5)
		aiMgrClient.Attempt(2)

		reply, err := aiMgrClient.Request(&message.Request{
			Command:    manager.IsServiceRunning,
			Parameters: datatype.New().Set("service", service.AiServiceName),
		})
		if err != nil {
			return req.Fail("ai manager request: " + err.Error())
		}
		if !reply.IsOK() {
			return req.Fail("ai manager: " + reply.ErrorMessage())
		}

		running, err := reply.ReplyParameters().BoolValue("running")
		fmt.Println("AI running: ", running)
		if err != nil {
			return req.Fail("ai running: " + err.Error())
		}
		if !running {
			return req.Fail("ai service is not running")
		}

		return req.Ok(map[string]any{"message": "hello " + name})
	}

	app.Route("hello", onHello)
	app.Route("age-verification", onAgeVerification)
	app.Route("country", onCountry)

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("Started and ready!")

	app.Wait()
}

func onAgeVerification(req service.RequestInterface) service.ReplyInterface {
	age, err := req.RouteParameters().Uint64Value("age")
	if err != nil {
		return req.Fail("age is required")
	}

	return req.Ok(map[string]any{"passed": age >= 18})
}

func onCountry(req service.RequestInterface) service.ReplyInterface {
	country, err := req.RouteParameters().StringValue("country")
	if err != nil {
		return req.Fail("country is required")
	}

	return req.Ok(map[string]any{"country": strings.ToUpper(country[0:2])})
}
