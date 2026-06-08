package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

var testEndpointSeq atomic.Uint64

func testEndpointID(t *testing.T, name string) string {
	t.Helper()
	seq := testEndpointSeq.Add(1)
	return fmt.Sprintf("%s_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), name, seq)
}

func testConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "noPerfection.json")
}

func closeTopologyHandler(t *testing.T) {
	t.Helper()

	controlConfig := control.CreateInternalConfig(topology.HandlerConfig())
	controlClient, err := clientSyncReplier.NewClient(controlConfig.Id, controlConfig.Port)
	if err == nil {
		_, _ = controlClient.Request(&message.Request{
			Command:    control.HandlerClose,
			Parameters: datatype.New(),
		})
		_ = controlClient.Close()
	}
	time.Sleep(100 * time.Millisecond)
}

func requireServiceHandler(t *testing.T, service topologyConfig.Service, category string) topologyConfig.Handler {
	t.Helper()

	handler, err := service.HandlerByCategory(category)
	require.NoError(t, err)
	return handler.AsHandler()
}

func TestNewDefaultParamsLintDefaultTopologyCreatesDefaultService(t *testing.T) {
	independent, err := New(nil, testConfigPath(t))
	require.NoError(t, err)
	require.Equal(t, DefaultName, independent.Name())

	require.NoError(t, independent.addDefaultServiceToTopology())

	serviceConfig, err := independent.topologyHandler.Service(DefaultName)
	require.NoError(t, err)
	require.Equal(t, topologyConfig.IndependentType, serviceConfig.Type)
	require.Empty(t, serviceConfig.Handlers)

	require.NoError(t, independent.addDefaultHandlerToTopology())

	serviceConfig, err = independent.topologyHandler.Service(DefaultName)
	require.NoError(t, err)
	require.Len(t, serviceConfig.Handlers, 1)

	defaultHandler := serviceConfig.Handlers[0].AsHandler()
	require.Equal(t, topologyConfig.ReplierType, defaultHandler.Type)
	require.Equal(t, handlers.DefaultHandlerCategory, defaultHandler.Category)
	require.Equal(t, handlers.DefaultHandlerEndpoint, defaultHandler.Endpoint)

	require.NoError(t, independent.addTopologyHandlersToHandlers())
	require.True(t, independent.Handlers.IsHandlerExist(handlers.DefaultHandlerCategory))
}

func TestNewUsesManagerEndpointFromConfigWhenEndpointNotPassed(t *testing.T) {
	configPath := testConfigPath(t)

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)
	require.Equal(t, DefaultServiceManagerEndpoint, independent.manager.Config().Endpoint)

	configuredEndpoint := message.NewEndpoint(testEndpointID(t, "configured-manager"), 0)
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
			Type:     topologyConfig.SyncReplierType,
			Category: topology.ServiceManagerCategory,
			Endpoint: configuredEndpoint,
		}),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	independent, err = New("custom-service", configPath)
	require.NoError(t, err)
	require.Equal(t, configuredEndpoint, independent.manager.Config().Endpoint)
}

func TestLintManagerTopologyOverwritesExistingManagerConfig(t *testing.T) {
	configPath := testConfigPath(t)
	existingManager := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: DefaultServiceManagerEndpoint,
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: topologyConfig.NewHandlerVariants(
			topologyConfig.Handler{
				Type:     topologyConfig.ReplierType,
				Category: handlers.DefaultHandlerCategory,
				Endpoint: handlers.DefaultHandlerEndpoint,
			},
			existingManager,
		),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	independent, err := New("custom-service", configPath, managerEndpoint)
	require.NoError(t, err)

	require.NoError(t, independent.addServiceManagerToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	managerHandler := requireServiceHandler(t, serviceConfig, topology.ServiceManagerCategory)
	require.Equal(t, topologyConfig.SyncReplierType, managerHandler.Type)
	require.Equal(t, managerEndpoint, managerHandler.Endpoint)
}

func TestLintDefaultTopologyKeepsExistingDefaultHandlerConfig(t *testing.T) {
	configPath := testConfigPath(t)
	existingMain := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "existing-main"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  topologyConfig.NewHandlerVariants(existingMain),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath, message.NewEndpoint(testEndpointID(t, "manager"), 0))
	require.NoError(t, err)

	require.NoError(t, independent.addDefaultServiceToTopology())
	require.NoError(t, independent.addDefaultHandlerToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	mainHandler := requireServiceHandler(t, serviceConfig, handlers.DefaultHandlerCategory)
	require.Equal(t, existingMain.Type, mainHandler.Type)
	require.Equal(t, existingMain.Endpoint, mainHandler.Endpoint)
}

func TestAddDefaultHandlerToTopologySkipsWhenHardcodedHandlersWereAdded(t *testing.T) {
	hardcodedMain := topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "hardcoded-main"), 0),
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerConfig(hardcodedMain))

	require.NoError(t, independent.addDefaultServiceToTopology())
	require.NoError(t, independent.addHardcodedHandlersToTopology())
	require.NoError(t, independent.addDefaultHandlerToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, topologyConfig.NewHandlerVariants(hardcodedMain), serviceConfig.Handlers)
}

func TestAddHardcodedServicesToTopologyAddsProxyService(t *testing.T) {
	proxyConfig := topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      "default-name-proxy",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.HandlerVariant{
			topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
				Handler: topologyConfig.Handler{
					Type:     topologyConfig.SyncReplierType,
					Category: "default-name",
					Endpoint: message.NewEndpoint(testEndpointID(t, "proxy"), 8001),
				},
				Outbounds: []topologyConfig.ServicePointer{
					topologyConfig.RefTarget("hello-world", handlers.DefaultHandlerCategory),
				},
			}),
		},
	}
	independent, err := New("hello-world", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(proxyConfig))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addDefaultServiceToTopology())

	actual, err := independent.topologyHandler.Service("default-name-proxy")
	require.NoError(t, err)
	require.Equal(t, proxyConfig, actual)
}

func TestAddHardcodedServicesToTopologyAddsServiceBeforeDefault(t *testing.T) {
	hardcodedMain := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "hardcoded-main"), 0),
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: "github.com/noPerfection/custom-service",
		Handlers:  topologyConfig.NewHandlerVariants(hardcodedMain),
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(serviceConfig))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addDefaultServiceToTopology())

	actual, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, serviceConfig, actual)
}

func TestAddHardcodedServicesToTopologyAllowsHardcodedHandlersForOtherService(t *testing.T) {
	hardcodedHandler := topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "other-service",
		ModuleUrl: DefaultModuleUrl,
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(serviceConfig))
	require.NoError(t, independent.SetHandlerConfig(hardcodedHandler, "other-service"))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addHardcodedHandlersToTopology())

	actual, err := independent.topologyHandler.Service("other-service")
	require.NoError(t, err)
	require.Equal(t, topologyConfig.NewHandlerVariants(hardcodedHandler), actual.Handlers)
}

func TestAddHardcodedCommandDepsToTopologyAddsDepsToDefaultHandler(t *testing.T) {
	dep := topologyConfig.DepService{
		Name:    "account",
		Proxies: []topologyConfig.ServicePointer{topologyConfig.RefTarget("account-proxy")},
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetCommandDeps(dep))

	require.NoError(t, independent.addDefaultServiceToTopology())
	require.NoError(t, independent.addDefaultHandlerToTopology())
	require.NoError(t, independent.addHardcodedCommandDepsToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	mainHandler := requireServiceHandler(t, serviceConfig, handlers.DefaultHandlerCategory)
	require.Equal(t, []topologyConfig.DepService{dep}, mainHandler.CommandDeps)
}

func TestAddHardcodedCommandDepsToTopologyAddsDepsToExplicitHandlerAndService(t *testing.T) {
	handler := topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
	}
	dep := topologyConfig.DepService{
		Name:       "metrics",
		Extensions: []topologyConfig.ServicePointer{topologyConfig.RefTarget("metrics-extension")},
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "other-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  topologyConfig.NewHandlerVariants(handler),
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(serviceConfig))
	require.NoError(t, independent.SetCommandDeps(dep, "api", "other-service"))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addHardcodedCommandDepsToTopology())

	actual, err := independent.topologyHandler.Service("other-service")
	require.NoError(t, err)
	apiHandler := requireServiceHandler(t, actual, "api")
	require.Equal(t, []topologyConfig.DepService{dep}, apiHandler.CommandDeps)
}

func TestAddHardcodedCommandDepsToTopologyRejectsMissingHandler(t *testing.T) {
	dep := topologyConfig.DepService{Name: "account"}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetCommandDeps(dep, "missing-handler"))

	require.NoError(t, independent.addDefaultServiceToTopology())

	err = independent.addHardcodedCommandDepsToTopology()
	require.Error(t, err)
	require.Contains(t, err.Error(), "hardcoded command deps handler 'missing-handler'")
}

func TestEnsureProxyHandlerOutboundAddsRouteAndOutboundHandler(t *testing.T) {
	proxyConfig := topologyConfig.ProxyHandler{
		Handler: topologyConfig.Handler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "proxy"), 0),
		},
		Routes: []string{"existing"},
		Outbounds: []topologyConfig.ServicePointer{
			topologyConfig.ServiceTarget(topologyConfig.Service{
				Type:      topologyConfig.IndependentType,
				Name:      "custom-service",
				ModuleUrl: DefaultModuleUrl,
				Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
					Type:     topologyConfig.ReplierType,
					Category: "api",
					Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
				}),
			}),
		},
	}
	webHandler := topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: "web",
		Endpoint: message.NewEndpoint(testEndpointID(t, "web"), 0),
	}
	outbound := topologyConfig.ServiceTarget(topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  topologyConfig.NewHandlerVariants(webHandler),
	})

	proxyConfig.Routes = appendUnique(proxyConfig.Routes, "hello")
	proxyConfig, changed := ensureProxyHandlerOutbound(proxyConfig, outbound)
	require.True(t, changed)
	require.ElementsMatch(t, []string{"existing", "hello"}, proxyConfig.Routes)
	require.Len(t, proxyConfig.Outbounds, 1)
	_, err := proxyConfig.Outbounds[0].Service.HandlerByCategory("api")
	require.NoError(t, err)
	_, err = proxyConfig.Outbounds[0].Service.HandlerByCategory("web")
	require.NoError(t, err)

	proxyConfig, changed = ensureProxyHandlerOutbound(proxyConfig, outbound)
	require.False(t, changed)
	require.Len(t, proxyConfig.Outbounds, 1)
	require.Len(t, proxyConfig.Outbounds[0].Service.Handlers, 2)
}

func TestAddHardcodedHandlerDepsToTopologyAddsDepsToDefaultService(t *testing.T) {
	dep := topologyConfig.DepService{
		Name:    "account",
		Proxies: []topologyConfig.ServicePointer{topologyConfig.RefTarget("account-proxy")},
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerDeps(dep))

	require.NoError(t, independent.addDefaultServiceToTopology())
	require.NoError(t, independent.addHardcodedHandlerDepsToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, []topologyConfig.DepService{dep}, serviceConfig.HandlerDeps)
}

func TestAddHardcodedHandlerDepsToTopologyAddsDepsToExplicitService(t *testing.T) {
	dep := topologyConfig.DepService{
		Name:       "metrics",
		Extensions: []topologyConfig.ServicePointer{topologyConfig.RefTarget("metrics-extension")},
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "other-service",
		ModuleUrl: DefaultModuleUrl,
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(serviceConfig))
	require.NoError(t, independent.SetHandlerDeps(dep, "other-service"))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addHardcodedHandlerDepsToTopology())

	actual, err := independent.topologyHandler.Service("other-service")
	require.NoError(t, err)
	require.Equal(t, []topologyConfig.DepService{dep}, actual.HandlerDeps)
}

func TestAddHardcodedHandlerDepsToTopologyRejectsMissingService(t *testing.T) {
	dep := topologyConfig.DepService{Name: "account"}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerDeps(dep, "missing-service"))

	require.NoError(t, independent.addDefaultServiceToTopology())

	err = independent.addHardcodedHandlerDepsToTopology()
	require.Error(t, err)
	require.Contains(t, err.Error(), "hardcoded handler deps for 'missing-service' service not found in topology")
}

func TestAddHardcodedHandlersToTopologyAddsHandlersToDefaultService(t *testing.T) {
	hardcodedMain := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "hardcoded-main"), 0),
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerConfig(hardcodedMain))

	require.NoError(t, independent.addDefaultServiceToTopology())
	require.NoError(t, independent.addHardcodedHandlersToTopology())
	require.NoError(t, independent.addDefaultHandlerToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, topologyConfig.NewHandlerVariants(hardcodedMain), serviceConfig.Handlers)
}

func TestAddHardcodedHandlersToTopologyOverwritesExistingCategory(t *testing.T) {
	configPath := testConfigPath(t)
	existingMain := topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "existing-main"), 0),
	}
	hardcodedMain := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "hardcoded-main"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  topologyConfig.NewHandlerVariants(existingMain),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerConfig(hardcodedMain))

	require.NoError(t, independent.addHardcodedHandlersToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, topologyConfig.NewHandlerVariants(hardcodedMain), serviceConfig.Handlers)
}

func TestAddHardcodedHandlersToTopologyRejectsMissingService(t *testing.T) {
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerConfig(topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "missing-service-main"), 0),
	}, "missing-service"))

	require.NoError(t, independent.addDefaultServiceToTopology())

	err = independent.addHardcodedHandlersToTopology()
	require.Error(t, err)
	require.Contains(t, err.Error(), "hardcoded handlers for 'missing-service' service not found in topology")
}

func TestAddTopologyHandlersRegistersServiceHandlersExceptManager(t *testing.T) {
	configPath := testConfigPath(t)
	mainHandler := topologyConfig.Handler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "main"), 0),
	}
	managerHandler := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "manager"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  topologyConfig.NewHandlerVariants(mainHandler, managerHandler),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath, managerHandler.Endpoint)
	require.NoError(t, err)

	require.NoError(t, independent.addTopologyHandlersToHandlers())

	require.True(t, independent.Handlers.IsHandlerExist(handlers.DefaultHandlerCategory))
	require.False(t, independent.Handlers.IsHandlerExist(topology.ServiceManagerCategory))
}

func TestStartCreatesDefaultHandlerAndStartsManager(t *testing.T) {
	independent, err := New(
		"custom-service",
		testConfigPath(t),
		message.NewEndpoint(testEndpointID(t, "manager"), 0),
	)
	require.NoError(t, err)

	require.NoError(t, independent.Start())
	t.Cleanup(func() {
		_ = independent.Stop()
		closeTopologyHandler(t)
	})

	require.True(t, independent.manager.Running())

	topologyClient, err := topology.NewClient()
	require.NoError(t, err)
	defer topologyClient.Close()

	serviceConfig, err := topologyClient.Service("custom-service")
	require.NoError(t, err)
	mainHandler := requireServiceHandler(t, serviceConfig, handlers.DefaultHandlerCategory)
	require.Equal(t, handlers.DefaultHandlerEndpoint, mainHandler.Endpoint)
}

func TestNewRejectsInvalidParams(t *testing.T) {
	_, err := New("service", testConfigPath(t), message.NewEndpoint("manager", 0), "extra")
	require.EqualError(t, err, "too many arguments, expected name, config path, and manager endpoint")

	_, err = New(10)
	require.EqualError(t, err, "name argument must be string")

	_, err = New("service", 10)
	require.EqualError(t, err, "config path argument must be string")

	_, err = New("service", testConfigPath(t), "manager")
	require.EqualError(t, err, "manager endpoint argument must be message.Endpoint")
}
