package manager

import (
	"testing"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func TestProxyManagerNormalizeOutboundRefUsesDefaultHandler(t *testing.T) {
	proxyServiceName := testEndpointID(t, "proxy")
	outboundServiceName := testEndpointID(t, "outbound")
	outboundHandler := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "outbound-handler"), 0),
	}
	outboundService := topologyConfig.Service{
		Type:      topologyConfig.ExtensionType,
		Name:      outboundServiceName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers:  topologyConfig.NewHandlerVariants(outboundHandler),
	}
	startTestRuntimeHandler(t, outboundService)

	topologyClient, err := topology.NewClient()
	require.NoError(t, err)
	defer topologyClient.Close()

	manager := &ProxyManager{
		serviceName:    proxyServiceName,
		topologyClient: topologyClient,
	}

	normalized, err := manager.normalizeProxyHandlerOutboundRef(topologyConfig.RefTarget(outboundServiceName))
	require.NoError(t, err)
	require.Empty(t, normalized.Ref)
	require.Equal(t, outboundServiceName, normalized.Service.Name)
	require.Len(t, normalized.Service.Handlers, 1)

	normalizedHandler := normalized.Service.Handlers[0].AsHandler()
	require.Equal(t, handlers.DefaultHandlerCategory, normalizedHandler.Category)
	require.Equal(t, proxyServiceName+"_proxy_"+outboundHandler.Endpoint.Id, normalizedHandler.Endpoint.Id)
}

func TestProxyManagerOnProxyHandlerRunningForwardsToProxyHandlers(t *testing.T) {
	serviceName := testEndpointID(t, "proxy")
	category := "default-name"

	proxyHandlers := handlers.NewProxyHandlers(serviceName)
	require.NoError(t, proxyHandlers.Route(base.Any, func(req handlers.ProxyRequest) handlers.ProxyReply {
		return handlers.ProxyReply{Reply: *req.Ok(datatype.New()).(*message.Reply)}
	}, category))
	require.NoError(t, proxyHandlers.Start())
	t.Cleanup(func() {
		_ = proxyHandlers.Close()
	})

	proxyHandlersClient, err := clientSyncReplier.NewClient(serviceName+handlers.ProxyHandlersCategory, 0)
	require.NoError(t, err)
	defer proxyHandlersClient.Close()

	manager := &ProxyManager{
		serviceName:         serviceName,
		proxyHandlersClient: proxyHandlersClient,
	}

	reply := manager.onSetProxyHandler(&message.Request{
		Command: handlers.SetProxyHandlerCommand,
		Parameters: datatype.New().
			Set("service", serviceName).
			Set("config", proxyHandlerConfigParams(t, validManagerProxyHandlerConfig(t, category))),
	})
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	reply = manager.onProxyHandlerRunning(&message.Request{
		Command: handlers.IsProxyHandlerRunningCommand,
		Parameters: datatype.New().
			Set("service", serviceName).
			Set("category", category),
	})
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	running, err := reply.ReplyParameters().BoolValue("running")
	require.NoError(t, err)
	require.False(t, running)
}

func TestProxyManagerOnProxyHandlerRunningValidatesServiceAndCategory(t *testing.T) {
	manager := &ProxyManager{serviceName: "proxy-service"}

	reply := manager.onProxyHandlerRunning(&message.Request{
		Command: handlers.IsProxyHandlerRunningCommand,
		Parameters: datatype.New().
			Set("service", "other-service").
			Set("category", "default-name"),
	})
	require.False(t, reply.IsOK())
	require.Equal(t, `service "other-service" does not match proxy service "proxy-service"`, reply.ErrorMessage())

	reply = manager.onProxyHandlerRunning(&message.Request{
		Command: handlers.IsProxyHandlerRunningCommand,
		Parameters: datatype.New().
			Set("service", "proxy-service"),
	})
	require.False(t, reply.IsOK())
	require.Equal(t, "req.RouteParameters().StringValue('category'): not exist", reply.ErrorMessage())
}

func TestProxyManagerOnSetProxyHandlerRequiresConfig(t *testing.T) {
	manager := &ProxyManager{serviceName: "proxy-service"}

	reply := manager.onSetProxyHandler(&message.Request{
		Command: handlers.SetProxyHandlerCommand,
		Parameters: datatype.New().
			Set("service", "proxy-service"),
	})
	require.False(t, reply.IsOK())
	require.Equal(t, "req.RouteParameters().NestedValue('config'): not exist", reply.ErrorMessage())
}

func validManagerProxyHandlerConfig(t *testing.T, category string) topologyConfig.ProxyHandler {
	t.Helper()

	return topologyConfig.ProxyHandler{
		Handler: topologyConfig.Handler{
			Type:     topologyConfig.SyncReplierType,
			Category: category,
			Endpoint: message.NewEndpoint(testEndpointID(t, category), 0),
		},
		Routes: []string{base.Any},
		Outbounds: []topologyConfig.ServicePointer{
			topologyConfig.ServiceTarget(topologyConfig.Service{
				Type:      topologyConfig.IndependentType,
				Name:      "outbound-" + category,
				ModuleUrl: "github.com/noPerfection/service/manager/test",
				Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
					Type:     topologyConfig.SyncReplierType,
					Category: handlers.DefaultHandlerCategory,
					Endpoint: message.NewEndpoint(testEndpointID(t, "outbound-"+category), 0),
				}),
			}),
		},
	}
}

func proxyHandlerConfigParams(t *testing.T, proxyConfig topologyConfig.ProxyHandler) datatype.KeyValue {
	t.Helper()

	params, err := datatype.NewFromInterface(proxyConfig)
	require.NoError(t, err)
	return params
}
