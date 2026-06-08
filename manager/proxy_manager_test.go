package manager

import (
	"testing"

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
