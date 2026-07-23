package manager

import (
	"testing"

	"github.com/noPerfection/datatype"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func testRouteURL(t *testing.T, serviceName, category, command string) string {
	t.Helper()
	link, err := mushroom.As("*pkg:$?var=services[name:"+serviceName+"]", category)
	require.NoError(t, err)
	return link.NewRouteURL(command).String()
}

func TestFilterTopologyInboundsExcludeInboundService(t *testing.T) {
	helloWorldName := testEndpointID(t, "hello-world")
	metricsName := testEndpointID(t, "metrics")
	entrypointName := testEndpointID(t, "entrypoint")

	helloWorldURL, err := mushroom.Parse("*pkg:$?var=services[name:" + helloWorldName + "]")
	require.NoError(t, err)
	metricsURL, err := mushroom.Parse("*pkg:$?var=services[name:" + metricsName + "]")
	require.NoError(t, err)

	metricsProtectedRoute := testRouteURL(t, metricsName, "main", message.Any)
	entrypointInboundRoute := testRouteURL(t, entrypointName, "main", message.Any)
	helloWorldInboundRoute := testRouteURL(t, helloWorldName, "main", "hello")

	topologyInbounds := map[string]map[string][]string{
		metricsURL.As(mushroom.SERVICE).String(): {
			metricsProtectedRoute: {entrypointInboundRoute, helloWorldInboundRoute},
		},
	}

	transitive, err := filterTopologyInbounds(topologyInbounds, metricsURL, helloWorldURL, true)
	require.NoError(t, err)
	require.Len(t, transitive, 1)
	require.Equal(t, entrypointInboundRoute, transitive[metricsProtectedRoute].String())

	direct, err := filterTopologyInbounds(topologyInbounds, metricsURL, helloWorldURL, false)
	require.NoError(t, err)
	require.Len(t, direct, 1)
	require.Equal(t, helloWorldInboundRoute, direct[metricsProtectedRoute].String())
}

func TestFilterTopologyOutboundsExcludeOutboundService(t *testing.T) {
	helloWorldName := testEndpointID(t, "hello-world")
	metricsName := testEndpointID(t, "metrics")
	proxyName := testEndpointID(t, "proxy")

	helloWorldURL, err := mushroom.Parse("*pkg:$?var=services[name:" + helloWorldName + "]")
	require.NoError(t, err)
	metricsURL, err := mushroom.Parse("*pkg:$?var=services[name:" + metricsName + "]")
	require.NoError(t, err)

	metricsHelloRoute := testRouteURL(t, metricsName, "main", "hello")
	metricsProxyRoute := testRouteURL(t, metricsName, "main", "proxy")
	helloWorldHelloRoute := testRouteURL(t, helloWorldName, "main", "hello")
	proxyHelloRoute := testRouteURL(t, proxyName, "main", "hello")

	topologyOutbounds := map[string]map[string]string{
		metricsURL.AsDereference().String(): {
			metricsHelloRoute:  helloWorldHelloRoute,
			metricsProxyRoute:  proxyHelloRoute,
		},
	}

	others, err := filterTopologyOutbounds(topologyOutbounds, metricsURL, helloWorldURL, true)
	require.NoError(t, err)
	require.Len(t, others, 1)
	require.Equal(t, proxyHelloRoute, others[metricsProxyRoute].String())

	self, err := filterTopologyOutbounds(topologyOutbounds, metricsURL, helloWorldURL, false)
	require.NoError(t, err)
	require.Len(t, self, 1)
	require.Equal(t, helloWorldHelloRoute, self[metricsHelloRoute].String())
}

func TestOnSecureEdgesDepsInDepsRequiresInbounds(t *testing.T) {
	_, _, metricsManager, _, _ := setupMetricsEntrypointManagers(t)
	reply := metricsManager.onSecureEdges(&message.Request{
		Command: SecureEdges,
		Parameters: datatype.New().
			Set(SecureEdgesProgressParam, SecureEdgesProgressDepsInDeps),
	})
	require.False(t, reply.IsOK())
	require.Contains(t, reply.ErrorMessage(), "inbounds")
}

func startFakeDepsInDepsEntrypointManager(t *testing.T, entrypointService topologyConfig.Service, metricsManager *Manager) {
	t.Helper()

	managerHandler, err := entrypointService.HandlerByCategory(topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	endpoint, ok := managerHandler.AsIndependentHandler()
	require.True(t, ok)

	handler := protocolHandler.NewSyncReplier()
	handler.SetEndpoint(endpoint.Endpoint)
	pub, sec, err := message.GenerateCurveKey()
	require.NoError(t, err)
	handler.Secure(sec)
	handler.Allow(metricsManager.PublicKey())
	for _, cmd := range managerWhitelistCommands() {
		handler.RequireWhitelist(cmd)
	}
	require.NoError(t, handler.Route(Handshake, func(req message.RequestInterface) message.ReplyInterface {
		secret, err := req.RouteParameters().StringValue("manager-hmac-secret")
		if err != nil {
			return req.Fail(err.Error())
		}
		outboundsRaw, err := req.RouteParameters().NestedValue("in-outbounds")
		if err != nil {
			return req.Fail(err.Error())
		}
		outbounds := make(map[string]RouteCredential)
		if err := outboundsRaw.Interface(&outbounds); err != nil {
			return req.Fail(err.Error())
		}
		replyOutbounds := make(map[string]string, len(outbounds))
		for _, cred := range outbounds {
			if cred.RouteURL == "" {
				continue
			}
			replyOutbounds[cred.RouteURL] = "test-entrypoint-main-pubkey"
		}
		for _, cmd := range managerWhitelistCommands() {
			if err := handler.Whitelist(cmd, secret); err != nil {
				return req.Fail(err.Error())
			}
		}
		return req.Ok(datatype.New().Set("inbounds", map[string]string{}).Set("outbounds", replyOutbounds))
	}))

	mushroomURL, err := mushroom.As("*pkg:$?var=services[name:"+entrypointService.Name+"]", topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	handler.SetMushroomURL(mushroomURL.String())
	require.NoError(t, handler.Start())

	publishManagerPublicKey(t, entrypointService.Name, pub)

	t.Cleanup(func() {
		_ = handlers.CloseViaControl(handler)
	})
}

func setupMetricsEntrypointManagers(t *testing.T) (
	metricsName string,
	entrypointName string,
	metricsManager *Manager,
	metricsProtectedRoute string,
	entrypointInboundRoute string,
) {
	t.Helper()

	metricsName = testEndpointID(t, "metrics")
	entrypointName = testEndpointID(t, "entrypoint")

	metricsService := calleeServiceConfig(t, metricsName)
	entrypointService := calleeServiceConfig(t, entrypointName)

	metricsManagerHandler, err := metricsService.HandlerByCategory(topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	metricsManagerEndpoint, ok := metricsManagerHandler.AsIndependentHandler()
	require.True(t, ok)

	entrypointManagerHandler, err := entrypointService.HandlerByCategory(topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	_, ok = entrypointManagerHandler.AsIndependentHandler()
	require.True(t, ok)

	startFakeServiceHandlers(t, metricsService)
	startFakeServiceHandlers(t, entrypointService)
	startTestRuntimeHandler(t, metricsService, entrypointService)

	metricsManager = newTestManager(t, metricsService, metricsManagerEndpoint.Endpoint)
	require.NoError(t, metricsManager.Start())

	startFakeDepsInDepsEntrypointManager(t, entrypointService, metricsManager)

	metricsServiceURL, err := mushroom.Parse("*pkg:$?var=services[name:" + metricsName + "]")
	require.NoError(t, err)

	publishManagerPublicKey(t, metricsName, metricsManager.PublicKey())
	allowManagerClient(t, entrypointName, metricsServiceURL, metricsManager.PublicKey())

	metricsProtectedRoute = testRouteURL(t, metricsName, "main", message.Any)
	entrypointInboundRoute = testRouteURL(t, entrypointName, "main", message.Any)
	return
}

func TestHandshakeCallerInboundDep(t *testing.T) {
	_, entrypointName, metricsManager, metricsProtectedRoute, entrypointInboundRoute := setupMetricsEntrypointManagers(t)

	entrypointDepURL, err := mushroom.Parse("*pkg:$?var=services[name:" + entrypointName + "]")
	require.NoError(t, err)

	require.NoError(t, metricsManager.handshakeCallerInboundDep(
		entrypointDepURL.AsDereference().String(),
		map[string]string{metricsProtectedRoute: entrypointInboundRoute},
	))
}

func TestOnSecureEdgesDepsInDeps(t *testing.T) {
	_, _, metricsManager, metricsProtectedRoute, entrypointInboundRoute := setupMetricsEntrypointManagers(t)

	inbounds, err := datatype.NewFromInterface(map[string]string{
		metricsProtectedRoute: entrypointInboundRoute,
	})
	require.NoError(t, err)

	reply := metricsManager.onSecureEdges(&message.Request{
		Command: SecureEdges,
		Parameters: datatype.New().
			Set(SecureEdgesProgressParam, SecureEdgesProgressDepsInDeps).
			Set("inbounds", inbounds),
	})
	require.True(t, reply.IsOK(), reply.ErrorMessage())
}

func TestFilterTopologyInboundsBuildsDepsInDepsPayload(t *testing.T) {
	helloWorldName := testEndpointID(t, "hello-world")
	metricsName := testEndpointID(t, "metrics")
	entrypointName := testEndpointID(t, "entrypoint")

	metricsProtectedRoute := testRouteURL(t, metricsName, "main", message.Any)
	entrypointInboundRoute := testRouteURL(t, entrypointName, "main", message.Any)
	helloWorldInboundRoute := testRouteURL(t, helloWorldName, "main", message.Any)

	metricsURL, err := mushroom.Parse("*pkg:$?var=services[name:" + metricsName + "]")
	require.NoError(t, err)
	helloWorldURL, err := mushroom.Parse("*pkg:$?var=services[name:" + helloWorldName + "]")
	require.NoError(t, err)

	topologyInbounds := map[string]map[string][]string{
		metricsURL.As(mushroom.SERVICE).String(): {
			metricsProtectedRoute: {entrypointInboundRoute, helloWorldInboundRoute},
		},
	}

	depInbounds, err := filterTopologyInbounds(topologyInbounds, metricsURL, helloWorldURL, true)
	require.NoError(t, err)

	inbounds := make(map[string]string, len(depInbounds))
	for route, inboundURL := range depInbounds {
		inbounds[route] = inboundURL.String()
	}

	require.Len(t, inbounds, 1)
	require.Equal(t, entrypointInboundRoute, inbounds[metricsProtectedRoute])
	require.Contains(t, inbounds[metricsProtectedRoute], "&category=main&command="+message.Any)
}
