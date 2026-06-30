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
	"github.com/noPerfection/protocol/handler/base"
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

func requireNewIndependent(t *testing.T, serviceName, configPath string) *Independent {
	t.Helper()
	independent, err := New(serviceName, configPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		closeTopologyHandler(t)
	})
	return independent
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

func requireServiceHandler(t *testing.T, service topologyConfig.Service, category string) topologyConfig.IndependentHandler {
	t.Helper()

	handler, err := service.HandlerByCategory(category)
	require.NoError(t, err)
	independentHandler, ok := handler.AsIndependentHandler()
	require.True(t, ok)
	return independentHandler
}

func TestNewDefaultParamsLintDefaultTopologyCreatesDefaultService(t *testing.T) {
	independent, err := New(nil, testConfigPath(t))
	require.NoError(t, err)
	require.Equal(t, DefaultName, independent.WithHardcodedTopology.mushroomURL)

	require.NoError(t, independent.addDefaultServiceToTopology())

	serviceConfig, err := independent.topologyHandler.Service(DefaultName)
	require.NoError(t, err)
	require.Equal(t, topologyConfig.IndependentType, serviceConfig.Type)
	require.Empty(t, serviceConfig.Handlers)

	require.NoError(t, independent.addDefaultHandlerToTopology())

	serviceConfig, err = independent.topologyHandler.Service(DefaultName)
	require.NoError(t, err)
	require.Len(t, serviceConfig.Handlers, 1)

	defaultHandler, ok := serviceConfig.Handlers[0].AsIndependentHandler()
	require.True(t, ok)
	require.Equal(t, topologyConfig.ReplierType, defaultHandler.Type)
	require.Equal(t, handlers.DefaultHandlerCategory, defaultHandler.Category)
	require.Equal(t, handlers.DefaultHandlerEndpoint, defaultHandler.Endpoint)

	require.NoError(t, independent.addTopologyHandlersToHandlers())
	require.True(t, independent.Handlers.IsHandlerExist(handlers.DefaultHandlerCategory))
}

func TestAddDefaultServiceToTopologyFillsModuleURL(t *testing.T) {
	stubBuildInfo(t, "example.com/app", true)
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)

	require.NoError(t, independent.addDefaultServiceToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, "example.com/app", serviceConfig.ModuleUrl)
}

func TestEnsureServiceManagerUsesEndpointFromConfig(t *testing.T) {
	t.Run("default endpoint when topology has no manager", func(t *testing.T) {
		configPath := testConfigPath(t)
		existingService := topologyConfig.Service{
			Type:      topologyConfig.IndependentType,
			Name:      "custom-service",
			ModuleUrl: DefaultModuleUrl,
		}
		appConfig, err := topologyConfig.Load(configPath)
		require.NoError(t, err)
		require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
		require.NoError(t, appConfig.Save())

		independent, err := New("custom-service", configPath)
		require.NoError(t, err)
		require.NoError(t, independent.ensureServiceManager())
		require.Equal(t, DefaultServiceManagerEndpoint, independent.manager.Config().Endpoint)
	})

	t.Run("configured endpoint from topology manager handler", func(t *testing.T) {
		configPath := testConfigPath(t)
		configuredEndpoint := message.NewEndpoint(testEndpointID(t, "configured-manager"), 0)
		existingService := topologyConfig.Service{
			Type:      topologyConfig.IndependentType,
			Name:      "custom-service",
			ModuleUrl: DefaultModuleUrl,
			Handlers: testHandlers(topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topology.ServiceManagerCategory,
				Endpoint: configuredEndpoint,
			}),
		}
		appConfig, err := topologyConfig.Load(configPath)
		require.NoError(t, err)
		require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
		require.NoError(t, appConfig.Save())

		independent, err := New("custom-service", configPath)
		require.NoError(t, err)
		require.NoError(t, independent.ensureServiceManager())
		require.Equal(t, configuredEndpoint, independent.manager.Config().Endpoint)
	})
}

func TestEnsureServiceManagerUsesExistingManagerFromTopology(t *testing.T) {
	configPath := testConfigPath(t)
	existingManager := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: DefaultServiceManagerEndpoint,
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: testHandlers(
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.ReplierType,
				Category: handlers.DefaultHandlerCategory,
				Endpoint: handlers.DefaultHandlerEndpoint,
			},
			existingManager,
		),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)

	require.NoError(t, independent.ensureServiceManager())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	managerHandler := requireServiceHandler(t, serviceConfig, topology.ServiceManagerCategory)
	require.Equal(t, topologyConfig.SyncReplierType, managerHandler.Type)
	require.Equal(t, DefaultServiceManagerEndpoint, managerHandler.Endpoint)
	require.Equal(t, DefaultServiceManagerEndpoint, independent.manager.Config().Endpoint)
}

func TestLintDefaultTopologyKeepsExistingDefaultHandlerConfig(t *testing.T) {
	configPath := testConfigPath(t)
	existingMain := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "existing-main"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  testHandlers(existingMain),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath)
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
	hardcodedMain := topologyConfig.IndependentHandler{
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
	require.Equal(t, testHandlers(hardcodedMain), serviceConfig.Handlers)
}

func TestAddHardcodedServicesToTopologyAddsProxyService(t *testing.T) {
	proxyConfig := topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      "default-name-proxy",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: "default-name",
					Endpoint: message.NewEndpoint(testEndpointID(t, "proxy"), 8001),
				},
				Outbounds: []string{outboundLink("hello-world", handlers.DefaultHandlerCategory)},
			},
		},
	}
	independent, err := New("hello-world", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(proxyConfig))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addDefaultServiceToTopology())

	actual, err := independent.topologyHandler.Service("default-name-proxy")
	require.NoError(t, err)
	requireEqualPersistedService(t, proxyConfig, actual)
}

func TestAddHardcodedServicesToTopologyAddsServiceBeforeDefault(t *testing.T) {
	hardcodedMain := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "hardcoded-main"), 0),
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: "github.com/noPerfection/custom-service",
		Handlers:  testHandlers(hardcodedMain),
	}
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(serviceConfig))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addDefaultServiceToTopology())

	actual, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	requireEqualPersistedService(t, serviceConfig, actual)
}

func TestAddHardcodedServicesToTopologyAllowsHardcodedHandlersForOtherService(t *testing.T) {
	hardcodedHandler := topologyConfig.IndependentHandler{
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
	require.Equal(t, testHandlers(hardcodedHandler), actual.Handlers)
}

func TestAddHardcodedCommandDepsToTopologyAddsDepsToDefaultHandler(t *testing.T) {
	dep := topologyConfig.DepService{
		Name:    "account",
		Proxies: []string{linkTarget("account-proxy")},
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
	handler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.ReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
	}
	dep := topologyConfig.DepService{
		Name:       "metrics",
		Extensions: []string{linkTarget("metrics-extension")},
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "other-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  testHandlers(handler),
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

func TestEnsureProxyHandlerOutboundAddsRouteAndOutboundURL(t *testing.T) {
	existingOutbound := outboundLink("custom-service", "api")
	proxyConfig := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "proxy"), 0),
		},
		Routes:    []string{"existing"},
		Outbounds: []string{existingOutbound},
	}
	newOutbound := outboundLink("custom-service", "web")

	proxyConfig.Routes = appendUnique(proxyConfig.Routes, "hello")
	changed := proxyConfig.SetOutbound(newOutbound)
	require.True(t, changed)
	require.ElementsMatch(t, []string{"existing", "hello"}, proxyConfig.Routes)
	require.Len(t, proxyConfig.Outbounds, 2)
	require.Contains(t, proxyConfig.Outbounds, existingOutbound)
	require.Contains(t, proxyConfig.Outbounds, newOutbound)

	changed = proxyConfig.SetOutbound(newOutbound)
	require.False(t, changed)
	require.Len(t, proxyConfig.Outbounds, 2)
}

func TestCommandOutboundTargetUsesFacadeURL(t *testing.T) {
	configPath := testConfigPath(t)
	apiHandler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  testHandlers(apiHandler),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(serviceConfig, rootServicesParent))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)
	outboundURL, err := independent.GetHandlerLink("api")
	require.NoError(t, err)
	require.Contains(t, outboundURL, "services[name:custom-service]&category=api")
}

func TestHandlerDepProxyOutboundTargetsUsesNextProxyThenCommandProxyForwards(t *testing.T) {
	configPath := testConfigPath(t)
	proxyService := func(name, endpointID string) topologyConfig.Service {
		return topologyConfig.Service{
			Type:      topologyConfig.ProxyType,
			Name:      name,
			ModuleUrl: DefaultModuleUrl,
			Handlers: []topologyConfig.Handler{
				topologyConfig.ProxyHandler{
					IndependentHandler: topologyConfig.IndependentHandler{
						Type:     topologyConfig.SyncReplierType,
						Category: handlers.DefaultHandlerCategory,
						Endpoint: message.NewEndpoint(testEndpointID(t, endpointID), 0),
					},
				},
			},
		}
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	for _, svc := range []topologyConfig.Service{
		proxyService("entrypoint", "entrypoint"),
		proxyService("audit", "audit"),
		proxyService("default-name-proxy", "default-name-proxy"),
	} {
		require.NoError(t, appConfig.AddService(svc, rootServicesParent))
	}

	apiHandler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
		CommandDeps: []topologyConfig.DepService{
			{
				Name:    "hello",
				Proxies: []string{linkTarget("default-name-proxy")},
			},
		},
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  testHandlers(apiHandler),
	}
	require.NoError(t, appConfig.AddService(serviceConfig, rootServicesParent))
	require.NoError(t, appConfig.Save())

	proxies := []string{linkTarget("entrypoint"), linkTarget("audit")}

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)
	outboundURL, commandOutbounds, err := independent.handlerDepProxyOutboundTargets(apiHandler, proxies, 0, []string{"age-verification", "hello"})
	require.NoError(t, err)
	require.Contains(t, outboundURL, "services[name:audit]&category="+handlers.DefaultHandlerCategory)
	require.Empty(t, commandOutbounds)

	outboundURL, commandOutbounds, err = independent.handlerDepProxyOutboundTargets(apiHandler, proxies, 1, []string{"age-verification", "hello"})
	require.NoError(t, err)
	require.Contains(t, outboundURL, "services[name:custom-service]&category=api")
	require.Len(t, commandOutbounds, 1)
	require.Contains(t, commandOutbounds["hello"], "services[name:default-name-proxy]")
}

func TestEnsureProxyHandlerForwardSetsCommandOutboundURL(t *testing.T) {
	proxyConfig := topologyConfig.ProxyHandler{}
	outboundURL := outboundLink("next-proxy", "proxy-api")

	proxyConfig, changed := ensureProxyHandlerForward(proxyConfig, "hello", outboundURL)
	require.True(t, changed)
	require.Equal(t, map[string]string{"hello": outboundURL}, proxyConfig.Forward)

	proxyConfig, changed = ensureProxyHandlerForward(proxyConfig, "hello", outboundURL)
	require.False(t, changed)
}

func TestValidateProtocolOrders(t *testing.T) {
	serviceTCP := func(t *testing.T) topologyConfig.Service {
		return protocolOutboundService(t, "service", topologyConfig.IndependentType, "tcp")
	}
	serviceInproc := func(t *testing.T) topologyConfig.Service {
		return protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc")
	}
	serviceIPC := func(t *testing.T) topologyConfig.Service {
		return protocolOutboundService(t, "service", topologyConfig.IndependentType, "ipc")
	}
	commandProxyIPC := func(t *testing.T) topologyConfig.Service {
		return protocolProxyService(t, "command-proxy", "ipc")
	}
	commandProxyInproc := func(t *testing.T) topologyConfig.Service {
		return protocolProxyService(t, "command-proxy", "inproc")
	}

	tests := []struct {
		name    string
		service topologyConfig.Service
		fixture func(t *testing.T) []topologyConfig.Service
		wantErr string
	}{
		{
			name: "inproc proxy to default tcp service passes",
			service: protocolProxyService(
				t,
				"proxy",
				"inproc",
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service { return []topologyConfig.Service{serviceTCP(t)} },
		},
		{
			name: "ipc proxy to default tcp service passes",
			service: protocolProxyService(
				t,
				"proxy",
				"ipc",
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service { return []topologyConfig.Service{serviceTCP(t)} },
		},
		{
			name: "ipc proxy to inproc service fails",
			service: protocolProxyService(
				t,
				"proxy",
				"ipc",
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service { return []topologyConfig.Service{serviceInproc(t)} },
			wantErr: "can not access from ipc to inproc",
		},
		{
			name: "inproc proxy to inproc service passes",
			service: protocolProxyService(
				t,
				"proxy",
				"inproc",
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service { return []topologyConfig.Service{serviceInproc(t)} },
		},
		{
			name: "inproc proxy to ipc proxy to tcp service passes",
			service: protocolProxyService(
				t,
				"proxy-a",
				"inproc",
				protocolOutboundLink("proxy-b"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{
					protocolProxyService(t, "proxy-b", "ipc", protocolOutboundLink("service")),
					serviceTCP(t),
				}
			},
		},
		{
			name: "tcp proxy to inproc proxy to ipc service fails",
			service: protocolProxyService(
				t,
				"proxy-a",
				"tcp",
				protocolOutboundLink("proxy-b"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{
					protocolProxyService(t, "proxy-b", "inproc", protocolOutboundLink("service")),
					serviceIPC(t),
				}
			},
			wantErr: "can not access from tcp to inproc",
		},
		{
			name: "inproc handler proxy with ipc command proxy and ipc service passes",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"inproc",
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyIPC(t), serviceIPC(t)}
			},
		},
		{
			name: "tcp handler proxy with ipc command proxy and inproc service fails at command proxy",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"tcp",
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyIPC(t), serviceInproc(t)}
			},
			wantErr: "can not access from tcp to ipc",
		},
		{
			name: "ipc handler proxy with ipc command proxy and inproc service fails at service",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"ipc",
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyIPC(t), serviceInproc(t)}
			},
			wantErr: "can not access from ipc to inproc",
		},
		{
			name: "tcp handler proxy with inproc command proxy and inproc service fails at command proxy",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"tcp",
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyInproc(t), serviceInproc(t)}
			},
			wantErr: "can not access from tcp to inproc",
		},
		{
			name: "tcp handler marked inproc can access inproc command proxy and service",
			service: protocolProxyServiceWithInprocHandlers(
				t,
				"handler-proxy",
				"tcp",
				[]string{handlers.DefaultHandlerCategory},
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyInproc(t), serviceInproc(t)}
			},
		},
		{
			name: "tcp handler marked ipc can access ipc command proxy and ipc service",
			service: protocolProxyServiceWithIpcHandlers(
				t,
				"handler-proxy",
				"tcp",
				[]string{handlers.DefaultHandlerCategory},
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyIPC(t), serviceIPC(t)}
			},
		},
		{
			name: "tcp handler marked ipc fails when accessing inproc service",
			service: protocolProxyServiceWithIpcHandlers(
				t,
				"handler-proxy",
				"tcp",
				[]string{handlers.DefaultHandlerCategory},
				protocolOutboundLink("command-proxy"),
				protocolOutboundLink("service"),
			),
			fixture: func(t *testing.T) []topologyConfig.Service {
				return []topologyConfig.Service{commandProxyIPC(t), serviceInproc(t)}
			},
			wantErr: "can not access from ipc to inproc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := append(tt.fixture(t), tt.service)
			cfg := setupProtocolValidationConfig(t, services...)
			err := cfg.ValidateProtocolOrdersFor(tt.service)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func protocolProxyService(t *testing.T, name string, protocol string, outbounds ...string) topologyConfig.Service {
	t.Helper()
	return protocolProxyServiceWithInprocHandlers(t, name, protocol, nil, outbounds...)
}

func protocolProxyServiceWithInprocHandlers(t *testing.T, name string, protocol string, inprocHandlers []string, outbounds ...string) topologyConfig.Service {
	t.Helper()
	return protocolProxyLikeServiceWithHandlerParameters(t, name, topologyConfig.ProxyType, protocol, inprocHandlers, nil, outbounds...)
}

func protocolProxyServiceWithIpcHandlers(t *testing.T, name string, protocol string, ipcHandlers []string, outbounds ...string) topologyConfig.Service {
	t.Helper()
	return protocolProxyLikeServiceWithHandlerParameters(t, name, topologyConfig.ProxyType, protocol, nil, ipcHandlers, outbounds...)
}

func protocolProxyLikeServiceWithInprocHandlers(t *testing.T, name string, serviceType topologyConfig.Type, protocol string, inprocHandlers []string, outbounds ...string) topologyConfig.Service {
	t.Helper()
	return protocolProxyLikeServiceWithHandlerParameters(t, name, serviceType, protocol, inprocHandlers, nil, outbounds...)
}

func protocolProxyLikeServiceWithHandlerParameters(t *testing.T, name string, serviceType topologyConfig.Type, protocol string, inprocHandlers, ipcHandlers []string, outbounds ...string) topologyConfig.Service {
	t.Helper()
	proxyHandler := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: protocolEndpoint(t, name, protocol),
		},
		Routes:    []string{base.Any},
		Outbounds: append([]string(nil), outbounds...),
	}
	return topologyConfig.Service{
		Type:      serviceType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		StartCommand: func() string {
			if protocol == "ipc" {
				return "/bin/true"
			}
			if len(ipcHandlers) > 0 {
				return "/bin/true"
			}
			return ""
		}(),
		Parameters: func() datatype.KeyValue {
			if len(inprocHandlers) == 0 && len(ipcHandlers) == 0 {
				return nil
			}
			params := datatype.New()
			if len(inprocHandlers) > 0 {
				params = params.Set(topologyConfig.InprocHandlersParameter, inprocHandlers)
			}
			if len(ipcHandlers) > 0 {
				params = params.Set(topologyConfig.IpcHandlersParameter, ipcHandlers)
			}
			return params
		}(),
		Handlers: []topologyConfig.Handler{
			proxyHandler,
		},
	}
}

func protocolOutboundLink(name string) string {
	return outboundLink(name, handlers.DefaultHandlerCategory)
}

func protocolOutboundService(t *testing.T, name string, serviceType topologyConfig.Type, protocol string) topologyConfig.Service {
	t.Helper()
	service := topologyConfig.Service{
		Type:      serviceType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: handlers.DefaultHandlerCategory,
				Endpoint: protocolEndpoint(t, name, protocol),
			},
		},
	}
	if protocol == "ipc" {
		service.StartCommand = "/bin/true"
	}
	return service
}

func requireEqualPersistedService(t *testing.T, expected, actual topologyConfig.Service) {
	t.Helper()
	require.Equal(t, expected.Type, actual.Type)
	require.Equal(t, expected.Name, actual.Name)
	require.Equal(t, expected.ModuleUrl, actual.ModuleUrl)
	require.Equal(t, expected.StartCommand, actual.StartCommand)
	require.Equal(t, expected.HandlerDeps, actual.HandlerDeps)
	require.Equal(t, expected.Parameters, actual.Parameters)
	require.Equal(t, len(expected.Handlers), len(actual.Handlers))
	for i := range expected.Handlers {
		require.Equal(t, expected.Handlers[i], actual.Handlers[i])
	}
}

func setupProtocolValidationConfig(t *testing.T, fixtures ...topologyConfig.Service) *topologyConfig.NoPerfection {
	t.Helper()
	configPath := testConfigPath(t)
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	for _, svc := range fixtures {
		require.NoError(t, appConfig.AddService(svc, rootServicesParent))
	}
	require.NoError(t, appConfig.Save())

	reloaded, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	return &reloaded
}

func setupProtocolValidationIndependent(t *testing.T, fixtures ...topologyConfig.Service) *Independent {
	t.Helper()
	configPath := testConfigPath(t)
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	for _, svc := range fixtures {
		require.NoError(t, appConfig.AddService(svc, rootServicesParent))
	}
	require.NoError(t, appConfig.Save())
	return requireNewIndependent(t, "protocol-test", configPath)
}

func protocolEndpoint(t *testing.T, name string, protocol string) message.Endpoint {
	t.Helper()
	switch protocol {
	case "inproc":
		return message.NewEndpoint(testEndpointID(t, name), 0)
	case "ipc":
		return message.NewEndpoint("tmp/"+testEndpointID(t, name), 0)
	case "tcp":
		return message.NewEndpoint("localhost", 8000+testEndpointSeq.Add(1))
	default:
		t.Fatalf("unknown protocol %q", protocol)
		return message.Endpoint{}
	}
}

func TestAddHardcodedHandlerDepsToTopologyAddsDepsToDefaultService(t *testing.T) {
	dep := topologyConfig.DepService{
		Name:    "account",
		Proxies: []string{linkTarget("account-proxy")},
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

func TestAddHardcodedServiceParamsToTopologyMergesParams(t *testing.T) {
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
	}))
	require.NoError(t, independent.SetServiceParams(datatype.New().Set("mode", "tutorial")))

	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.addHardcodedServiceParamsToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, "tutorial", serviceConfig.Parameters["mode"])
}

func TestNewAiServiceRegistersInTopology(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "topology-test-key")

	cfgPath := testConfigPath(t)
	ai, err := NewAiService(cfgPath)
	require.NoError(t, err)

	topologyHandler, err := newTopologyHandler(cfgPath)
	require.NoError(t, err)

	aiService, err := topologyHandler.Service(AiServiceName)
	require.NoError(t, err)
	require.Equal(t, topologyConfig.ExtensionType, aiService.Type)
	require.Equal(t, AiServiceName, aiService.Name)
	require.Equal(t, defaultAiModel, aiService.Parameters[aiModelParameter])
	require.Equal(t, "topology-test-key", aiService.Parameters[aiAPIKeyParameter])
	_ = ai
}

func TestAddHardcodedHandlerDepsToTopologyAddsDepsToExplicitService(t *testing.T) {
	dep := topologyConfig.DepService{
		Name:       "metrics",
		Extensions: []string{linkTarget("metrics-extension")},
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
	require.Contains(t, err.Error(), `hardcoded handler deps for "missing-service" not found in topology`)
}

func TestAddHardcodedHandlersToTopologyAddsHandlersToDefaultService(t *testing.T) {
	hardcodedMain := topologyConfig.IndependentHandler{
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
	require.Equal(t, testHandlers(hardcodedMain), serviceConfig.Handlers)
}

func TestAddHardcodedHandlersToTopologyOverwritesExistingCategory(t *testing.T) {
	configPath := testConfigPath(t)
	existingMain := topologyConfig.IndependentHandler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "existing-main"), 0),
	}
	hardcodedMain := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "hardcoded-main"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  testHandlers(existingMain),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerConfig(hardcodedMain))

	require.NoError(t, independent.addHardcodedHandlersToTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	require.Equal(t, testHandlers(hardcodedMain), serviceConfig.Handlers)
}

func TestAddHardcodedHandlersToTopologyRejectsMissingService(t *testing.T) {
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetHandlerConfig(topologyConfig.IndependentHandler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "missing-service-main"), 0),
	}, "missing-service"))

	require.NoError(t, independent.addDefaultServiceToTopology())

	err = independent.addHardcodedHandlersToTopology()
	require.Error(t, err)
	require.Contains(t, err.Error(), `hardcoded handlers for "missing-service" not found in topology`)
}

func TestAddTopologyHandlersRegistersServiceHandlersExceptManager(t *testing.T) {
	configPath := testConfigPath(t)
	mainHandler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.ReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "main"), 0),
	}
	managerHandler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "manager"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  testHandlers(mainHandler, managerHandler),
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath)
	require.NoError(t, err)

	require.NoError(t, independent.addTopologyHandlersToHandlers())

	require.True(t, independent.Handlers.IsHandlerExist(handlers.DefaultHandlerCategory))
	require.False(t, independent.Handlers.IsHandlerExist(topology.ServiceManagerCategory))
}

func TestStartCreatesDefaultHandlerAndStartsManager(t *testing.T) {
	independent, err := New(
		"custom-service",
		testConfigPath(t),
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
	_, err := New("service", testConfigPath(t), "extra")
	require.EqualError(t, err, "too many arguments, expected name and config path")

	_, err = New(10)
	require.EqualError(t, err, "name argument must be string")

	_, err = New("service", 10)
	require.EqualError(t, err, "config path argument must be string")
}

func TestStartIpcServiceSkipsDuplicateRefs(t *testing.T) {
	urls := []string{
		linkTarget("entrypoint"),
		linkTarget("entrypoint"),
	}
	startedRefs := make(map[string]struct{})
	for _, url := range urls {
		const linkPrefix = "pkg:$?var=services[name:"
		serviceName := strings.TrimSuffix(strings.TrimPrefix(url, linkPrefix), "]")
		if _, done := startedRefs[serviceName]; done {
			continue
		}
		startedRefs[serviceName] = struct{}{}
	}
	require.Len(t, startedRefs, 1)
}

func TestStartIpcServiceRequiresStartCommand(t *testing.T) {
	service := topologyConfig.Service{Name: "ipc-proxy"}
	require.Empty(t, service.StartCommand)
	require.EqualError(
		t,
		fmt.Errorf("service '%s' has no start command given", service.Name),
		"service 'ipc-proxy' has no start command given",
	)
}
