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

	independent, err = New("custom-service", configPath)
	require.NoError(t, err)
	require.Equal(t, configuredEndpoint, independent.manager.Config().Endpoint)
}

func TestLintManagerTopologyOverwritesExistingManagerConfig(t *testing.T) {
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
				Outbounds: []topologyConfig.Service{
					{
						Type: topologyConfig.IndependentType,
						Name: "hello-world",
						Handlers: testHandlers(topologyConfig.IndependentHandler{
							Type:     topologyConfig.ReplierType,
							Category: handlers.DefaultHandlerCategory,
							Endpoint: message.NewEndpoint(testEndpointID(t, "hello-world"), 0),
						}),
					},
				},
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
	require.Equal(t, proxyConfig, actual)
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
	require.Equal(t, serviceConfig, actual)
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
	handler := topologyConfig.IndependentHandler{
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

func TestEnsureProxyHandlerOutboundAddsRouteAndOutboundHandler(t *testing.T) {
	proxyConfig := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "proxy"), 0),
		},
		Routes: []string{"existing"},
		Outbounds: []topologyConfig.Service{
			{
				Type:      topologyConfig.IndependentType,
				Name:      "custom-service",
				ModuleUrl: DefaultModuleUrl,
				Handlers: []topologyConfig.Handler{
					topologyConfig.IndependentHandler{
						Type:     topologyConfig.ReplierType,
						Category: "api",
						Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
					},
				},
			},
		},
	}
	outbound := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.ReplierType,
				Category: "web",
				Endpoint: message.NewEndpoint(testEndpointID(t, "web"), 0),
			},
		},
	}

	proxyConfig.Routes = appendUnique(proxyConfig.Routes, "hello")
	proxyConfig, changed := ensureProxyHandlerOutbound(proxyConfig, outbound)
	require.True(t, changed)
	require.ElementsMatch(t, []string{"existing", "hello"}, proxyConfig.Routes)
	require.Len(t, proxyConfig.Outbounds, 1)
	require.Empty(t, proxyConfig.Outbounds[0].ModuleUrl)
	require.Empty(t, proxyConfig.Outbounds[0].HandlerDeps)
	handler := requireServiceHandler(t, proxyConfig.Outbounds[0], "web")
	require.Empty(t, handler.CommandDeps)
	_, err := proxyConfig.Outbounds[0].HandlerByCategory("api")
	require.Error(t, err)

	proxyConfig, changed = ensureProxyHandlerOutbound(proxyConfig, outbound)
	require.False(t, changed)
	require.Len(t, proxyConfig.Outbounds, 1)
	require.Len(t, proxyConfig.Outbounds[0].Handlers, 1)
}

func TestCommandOutboundTargetUsesCommandHandler(t *testing.T) {
	apiHandler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: testHandlers(apiHandler),
	}

	outbound := minimalOutboundService(serviceConfig, apiHandler)
	require.Equal(t, "custom-service", outbound.Name)
	require.Empty(t, outbound.ModuleUrl)
	require.Empty(t, outbound.HandlerDeps)
	require.Len(t, outbound.Handlers, 1)
	handler := requireServiceHandler(t, outbound, "api")
	require.Equal(t, apiHandler.Endpoint, handler.Endpoint)
	require.Empty(t, handler.CommandDeps)
}

func TestHandlerDepProxyOutboundTargetsUsesNextProxyThenCommandProxyForwards(t *testing.T) {
	apiHandler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
		CommandDeps: []topologyConfig.DepService{
			{
				Name: "hello",
				Proxies: []topologyConfig.ServicePointer{
					topologyConfig.ServiceTarget(topologyConfig.Service{
						Type:      topologyConfig.ProxyType,
						Name:      "default-name-proxy",
						ModuleUrl: DefaultModuleUrl,
						Handlers: []topologyConfig.Handler{
							topologyConfig.IndependentHandler{
								Type:     topologyConfig.SyncReplierType,
								Category: handlers.DefaultHandlerCategory,
								Endpoint: message.NewEndpoint(testEndpointID(t, "default-name-proxy"), 0),
							},
						},
					}),
				},
			},
		},
	}
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: testHandlers(apiHandler),
	}
	entrypoint := topologyConfig.ServiceTarget(topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      "entrypoint",
		ModuleUrl: DefaultModuleUrl,
		Handlers: testHandlers(topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "entrypoint"), 0),
		}),
	})
	audit := topologyConfig.ServiceTarget(topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      "audit",
		ModuleUrl: DefaultModuleUrl,
		Handlers: testHandlers(topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "audit"), 0),
		}),
	})
	proxies := []topologyConfig.ServicePointer{entrypoint, audit}

	independent := &Independent{}
	outbound, commandOutbounds, err := independent.handlerDepProxyOutboundTargets(serviceConfig, apiHandler, proxies, 0, []string{"age-verification", "hello"})
	require.NoError(t, err)
	require.Equal(t, "audit", outbound.Name)
	require.Empty(t, commandOutbounds)

	outbound, commandOutbounds, err = independent.handlerDepProxyOutboundTargets(serviceConfig, apiHandler, proxies, 1, []string{"age-verification", "hello"})
	require.NoError(t, err)
	require.Equal(t, "custom-service", outbound.Name)
	require.Len(t, commandOutbounds, 1)
	require.Equal(t, "default-name-proxy", commandOutbounds["hello"].Name)
}

func TestConfigureHandlerDepProxyConfigSetsRoutesOutboundsAndForwards(t *testing.T) {
	proxyConfig := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "entrypoint"), 0),
		},
		Routes:  []string{"stale"},
		Forward: map[string]string{"stale": "old/main"},
	}
	serviceOutbound := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: "api",
				Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
			},
		},
	}
	commandOutbound := topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      "default-name-proxy",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: handlers.DefaultHandlerCategory,
				Endpoint: message.NewEndpoint(testEndpointID(t, "default-name-proxy"), 0),
			},
		},
	}

	proxyConfig, changed, err := configureHandlerDepProxyConfig(proxyConfig, []string{"age-verification", "hello"}, serviceOutbound, map[string]topologyConfig.Service{
		"hello": commandOutbound,
	})
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"age-verification", "hello"}, proxyConfig.Routes)
	require.Equal(t, map[string]string{"hello": "default-name-proxy/main"}, proxyConfig.Forward)
	require.Len(t, proxyConfig.Outbounds, 2)
}

func TestEnsureProxyHandlerForwardSetsCommandOutboundRef(t *testing.T) {
	proxyConfig := topologyConfig.ProxyHandler{}
	outbound := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "next-proxy",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: "proxy-api",
				Endpoint: message.NewEndpoint(testEndpointID(t, "proxy-api"), 0),
			},
		},
	}

	proxyConfig, changed, err := ensureProxyHandlerForward(proxyConfig, "hello", outbound)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"hello": "next-proxy/proxy-api"}, proxyConfig.Forward)

	proxyConfig, changed, err = ensureProxyHandlerForward(proxyConfig, "hello", outbound)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestValidateProtocolOrders(t *testing.T) {
	tests := []struct {
		name    string
		service topologyConfig.Service
		wantErr string
	}{
		{
			name: "inproc proxy to default tcp service passes",
			service: protocolProxyService(
				t,
				"proxy",
				"inproc",
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "tcp"),
			),
		},
		{
			name: "ipc proxy to default tcp service passes",
			service: protocolProxyService(
				t,
				"proxy",
				"ipc",
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "tcp"),
			),
		},
		{
			name: "ipc proxy to inproc service fails",
			service: protocolProxyService(
				t,
				"proxy",
				"ipc",
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
			wantErr: "can not access from ipc to inproc",
		},
		{
			name: "inproc proxy to inproc service passes",
			service: protocolProxyService(
				t,
				"proxy",
				"inproc",
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
		},
		{
			name: "inproc proxy to ipc proxy to tcp service passes",
			service: protocolProxyService(
				t,
				"proxy-a",
				"inproc",
				protocolProxyService(
					t,
					"proxy-b",
					"ipc",
					protocolOutboundService(t, "service", topologyConfig.IndependentType, "tcp"),
				),
			),
		},
		{
			name: "tcp proxy to inproc proxy to ipc service fails",
			service: protocolProxyService(
				t,
				"proxy-a",
				"tcp",
				protocolProxyService(
					t,
					"proxy-b",
					"inproc",
					protocolOutboundService(t, "service", topologyConfig.IndependentType, "ipc"),
				),
			),
			wantErr: "can not access from tcp to inproc",
		},
		{
			name: "inproc handler proxy with ipc command proxy and ipc service passes",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"inproc",
				protocolOutboundService(t, "command-proxy", topologyConfig.ProxyType, "ipc"),
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "ipc"),
			),
		},
		{
			name: "tcp handler proxy with ipc command proxy and inproc service fails at command proxy",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"tcp",
				protocolOutboundService(t, "command-proxy", topologyConfig.ProxyType, "ipc"),
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
			wantErr: "can not access from tcp to ipc",
		},
		{
			name: "ipc handler proxy with ipc command proxy and inproc service fails at service",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"ipc",
				protocolOutboundService(t, "command-proxy", topologyConfig.ProxyType, "ipc"),
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
			wantErr: "can not access from ipc to inproc",
		},
		{
			name: "tcp handler proxy with inproc command proxy and inproc service fails at command proxy",
			service: protocolProxyService(
				t,
				"handler-proxy",
				"tcp",
				protocolOutboundService(t, "command-proxy", topologyConfig.ProxyType, "inproc"),
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
			wantErr: "can not access from tcp to inproc",
		},
		{
			name: "tcp handler marked inproc can access inproc command proxy and service",
			service: protocolProxyServiceWithInprocHandlers(
				t,
				"handler-proxy",
				"tcp",
				[]string{handlers.DefaultHandlerCategory},
				protocolOutboundService(t, "command-proxy", topologyConfig.ProxyType, "inproc"),
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
		},
		{
			name: "extension handler marked inproc can access inproc service",
			service: protocolProxyLikeServiceWithInprocHandlers(
				t,
				"extension",
				topologyConfig.ExtensionType,
				"tcp",
				[]string{handlers.DefaultHandlerCategory},
				protocolOutboundService(t, "service", topologyConfig.IndependentType, "inproc"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Independent{}).validateProtocolOrdersFor(tt.service)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func protocolProxyService(t *testing.T, name string, protocol string, outbounds ...topologyConfig.Service) topologyConfig.Service {
	t.Helper()
	return protocolProxyServiceWithInprocHandlers(t, name, protocol, nil, outbounds...)
}

func protocolProxyServiceWithInprocHandlers(t *testing.T, name string, protocol string, inprocHandlers []string, outbounds ...topologyConfig.Service) topologyConfig.Service {
	t.Helper()
	return protocolProxyLikeServiceWithInprocHandlers(t, name, topologyConfig.ProxyType, protocol, inprocHandlers, outbounds...)
}

func protocolProxyLikeServiceWithInprocHandlers(t *testing.T, name string, serviceType topologyConfig.Type, protocol string, inprocHandlers []string, outbounds ...topologyConfig.Service) topologyConfig.Service {
	t.Helper()
	proxyHandler := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: protocolEndpoint(t, name, protocol),
		},
		Routes: []string{base.Any},
	}
	for _, outbound := range outbounds {
		proxyHandler.Outbounds = append(proxyHandler.Outbounds, outbound)
	}
	return topologyConfig.Service{
		Type:      serviceType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		Parameters: func() datatype.KeyValue {
			if len(inprocHandlers) == 0 {
				return nil
			}
			return datatype.New().Set(topologyConfig.InprocHandlersParameter, inprocHandlers)
		}(),
		Handlers: []topologyConfig.Handler{
			proxyHandler,
		},
	}
}

func protocolOutboundService(t *testing.T, name string, serviceType topologyConfig.Type, protocol string) topologyConfig.Service {
	t.Helper()
	return topologyConfig.Service{
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
	require.Contains(t, err.Error(), "hardcoded handlers for 'missing-service' service not found in topology")
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

func TestStartIpcServiceSkipsDuplicateRefs(t *testing.T) {
	pointers := []topologyConfig.ServicePointer{
		topologyConfig.RefTarget("entrypoint"),
		topologyConfig.RefTarget("entrypoint"),
	}
	startedRefs := make(map[string]struct{})
	for _, pointer := range pointers {
		if pointer.Ref == "" {
			continue
		}
		serviceName, _ := pointer.RefPath()
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
