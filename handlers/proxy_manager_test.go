package handlers

import (
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/message"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func TestProxyHandlersLifecycle(t *testing.T) {
	require.Panics(t, func() {
		NewProxyHandlers("tmp-proxy-service")
	})

	var zero ProxyHandlers
	require.EqualError(t, zero.Start(), "proxy manager interface is nil, please create this manager using NewProxyHandlers(serviceName)")
	require.EqualError(t, zero.Close(), "proxy manager interface is nil, please create this manager using NewProxyHandlers(serviceName)")

	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager"))
	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		_ = manager.Close()
	})

	managerControl, err := clientSyncReplier.NewControl(
		control.ControlEndpointID(manager.Interface.Config().Id, manager.Interface.Config().Port),
		0,
	)
	require.NoError(t, err)
	managerControl.Timeout(time.Second)
	managerControl.Attempt(3)
	defer managerControl.Close()

	requireProxyManagerStatus(t, managerControl, base.SocketReady)
	require.Error(t, manager.Start())

	require.NoError(t, manager.Close())
	requireProxyManagerStatus(t, managerControl, base.SocketNil)
	require.NoError(t, manager.Start())
	requireProxyManagerStatus(t, managerControl, base.SocketReady)

	require.NoError(t, manager.Close())
	requireProxyManagerStatus(t, managerControl, base.SocketNil)
}

func requireProxyManagerStatus(t *testing.T, managerControl *clientSyncReplier.Control, expected string) {
	t.Helper()

	require.Eventually(t, func() bool {
		status, err := managerControl.HandlerStatus()
		return err == nil && status == expected
	}, 2*time.Second, 10*time.Millisecond)
}

func TestValidateProxyHandlerOutboundsRequiresInlineServiceWithHandler(t *testing.T) {
	inlineService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "api",
		ModuleUrl: "github.com/noPerfection/service/handlers/test",
		Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
			Type:     topologyConfig.ReplierType,
			Category: "main",
			Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
		}),
	}

	tests := []struct {
		name        string
		outbounds   []topologyConfig.ServicePointer
		expectedErr string
	}{
		{
			name:        "empty",
			outbounds:   nil,
			expectedErr: "not possible to send since no outbound yet",
		},
		{
			name:        "service-2/main",
			outbounds:   []topologyConfig.ServicePointer{topologyConfig.RefTarget("api")},
			expectedErr: `outbounds[0] must be inline service, not ref "api"`,
		},
		{
			name:        "missing service",
			outbounds:   []topologyConfig.ServicePointer{{}},
			expectedErr: "outbounds[0] service is required",
		},
		{
			name: "service without handler",
			outbounds: []topologyConfig.ServicePointer{
				topologyConfig.ServiceTarget(topologyConfig.Service{
					Type: topologyConfig.IndependentType,
					Name: "api",
				}),
			},
			expectedErr: `outbounds[0] service "api" must have at least one handler`,
		},
		{
			name:      "inline service with handler",
			outbounds: []topologyConfig.ServicePointer{topologyConfig.ServiceTarget(inlineService)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateProxyHandlerOutbounds(topologyConfig.ProxyHandler{
				Outbounds: test.outbounds,
			})
			if test.expectedErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.expectedErr)
		})
	}
}

func TestProxyHandlersSetProxyHandler(t *testing.T) {
	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-set"))
	category := "api"
	proxyConfig := validProxyHandlerConfig(t, category)
	require.NoError(t, manager.Route("hello", proxyOKRoute, category))

	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		_ = manager.Close()
	})

	client, err := clientSyncReplier.NewClient(manager.Interface.Config().Id, manager.Interface.Config().Port)
	require.NoError(t, err)
	client.Timeout(time.Second)
	client.Attempt(3)
	defer client.Close()

	reply := proxyManagerRequest(t, client, SetProxyHandlerCommand, datatype.New())
	require.False(t, reply.IsOK())
	require.Equal(t, "req.RouteParameters().NestedValue('config'): not exist", reply.ErrorMessage())

	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, datatype.New().Set("config", datatype.New().Set("type", 10)))
	require.False(t, reply.IsOK())
	require.Contains(t, reply.ErrorMessage(), "Can not convert 'config' to noPerfection/topology/config.ProxyHandler: ")

	noOutbounds := proxyConfig
	noOutbounds.Outbounds = []topologyConfig.ServicePointer{}
	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, proxyManagerConfigParams(t, noOutbounds))
	require.False(t, reply.IsOK())
	require.Equal(t, "not possible to send since no outbound yet", reply.ErrorMessage())

	noConfigRoutes := proxyConfig
	noConfigRoutes.Routes = nil
	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, proxyManagerConfigParams(t, noConfigRoutes))
	require.False(t, reply.IsOK())
	require.Equal(t, "not possible to send since no routes are configured", reply.ErrorMessage())

	reply = proxyManagerRequest(t, client, IsProxyHandlerRunningCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "No proxified handler was set, please call set-proxy-handler-command command to set it first", reply.ErrorMessage())
	requireProxyHandlerExists(t, client, category, false)

	withConfigRoutes := proxyConfig
	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, proxyManagerConfigParams(t, withConfigRoutes))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	requireProxyHandlerExists(t, client, category, true)
	requireProxyHandlerRunning(t, client, category, false)

	reply = proxyManagerRequest(t, client, RemoveProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	requireProxyHandlerExists(t, client, category, false)
	reply = proxyManagerRequest(t, client, IsProxyHandlerRunningCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "No proxified handler was set, please call set-proxy-handler-command command to set it first", reply.ErrorMessage())

	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, proxyManagerConfigParams(t, withConfigRoutes))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	requireProxyHandlerExists(t, client, category, true)
	requireProxyHandlerRunning(t, client, category, false)
}

func TestProxyHandlersStartStopProxyHandler(t *testing.T) {
	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-start-stop"))
	category := "api"
	proxyConfig := validProxyHandlerConfig(t, category)
	require.NoError(t, manager.Route("hello", proxyOKRoute, category))

	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		_ = manager.Close()
	})

	managerClient, err := clientSyncReplier.NewClient(manager.Interface.Config().Id, manager.Interface.Config().Port)
	require.NoError(t, err)
	managerClient.Timeout(time.Second)
	managerClient.Attempt(3)
	defer managerClient.Close()

	reply := proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "No proxified handler was set, please call set-proxy-handler-command command to set it first", reply.ErrorMessage())

	reply = proxyManagerRequest(t, managerClient, SetProxyHandlerCommand, proxyManagerConfigParams(t, proxyConfig))
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	reply = proxyManagerRequest(t, managerClient, StopProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "proxified handler is not running", reply.ErrorMessage())

	reply = proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	requireProxyHandlerRunning(t, managerClient, category, true)

	reply = proxyManagerRequest(t, managerClient, SetProxyHandlerCommand, proxyManagerConfigParams(t, proxyConfig))
	require.False(t, reply.IsOK())
	require.Equal(t, "not possible to send since the handler is already running, stop", reply.ErrorMessage())

	reply = proxyManagerRequest(t, managerClient, RemoveProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "proxified handler is running, stop it first", reply.ErrorMessage())

	proxyClient := newProxyHandlerClient(t, proxyConfig)
	defer proxyClient.Close()
	requireProxifiedReply(t, proxyClient, "abracadavarda")

	reply = proxyManagerRequest(t, managerClient, StopProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	reply = proxyManagerRequest(t, managerClient, StopProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "proxified handler is not running", reply.ErrorMessage())
	requireProxyHandlerRunning(t, managerClient, category, false)
	requireProxyHandlerRequestTimeout(t, proxyConfig, "abracadavarda")

	reply = proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	afterRestartClient := newProxyHandlerClient(t, proxyConfig)
	defer afterRestartClient.Close()
	requireProxifiedReply(t, afterRestartClient, "abracadavarda")

	beforeCloseClient := newProxyHandlerClient(t, proxyConfig)
	defer beforeCloseClient.Close()
	requireProxifiedReply(t, beforeCloseClient, "anything-before-close")

	require.NoError(t, manager.Close())
	requireProxyHandlerRequestTimeout(t, proxyConfig, "anything-after-close")
}

func proxyOKRoute(req ProxyRequest) ProxyReply {
	return ProxyReply{Reply: *req.Ok(datatype.New()).(*message.Reply)}
}

func validProxyHandlerConfig(t *testing.T, category string) topologyConfig.ProxyHandler {
	t.Helper()

	return topologyConfig.ProxyHandler{
		Handler: topologyConfig.Handler{
			Type:     topologyConfig.SyncReplierType,
			Category: category,
			Endpoint: message.NewEndpoint(testEndpointID(t, category), 0),
		},
		Routes: []string{"hello"},
		Outbounds: []topologyConfig.ServicePointer{
			topologyConfig.ServiceTarget(topologyConfig.Service{
				Type:      topologyConfig.IndependentType,
				Name:      "outbound-" + category,
				ModuleUrl: "github.com/noPerfection/service/handlers/test",
				Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
					Type:     topologyConfig.SyncReplierType,
					Category: DefaultHandlerCategory,
					Endpoint: message.NewEndpoint(testEndpointID(t, "outbound-"+category), 0),
				}),
			}),
		},
	}
}

func proxyManagerConfigParams(t *testing.T, proxyConfig topologyConfig.ProxyHandler) datatype.KeyValue {
	t.Helper()

	configParams, err := datatype.NewFromInterface(proxyConfig)
	require.NoError(t, err)
	return datatype.New().Set("config", configParams)
}

func proxyManagerCategoryParams(category string) datatype.KeyValue {
	return datatype.New().Set("category", category)
}

func proxyManagerRequest(t *testing.T, client *clientSyncReplier.Client, command string, params datatype.KeyValue) message.ReplyInterface {
	t.Helper()

	reply, err := client.Request(&message.Request{
		Command:    command,
		Parameters: params,
	})
	require.NoError(t, err)
	return reply
}

func requireProxyHandlerExists(t *testing.T, client *clientSyncReplier.Client, category string, expected bool) {
	t.Helper()

	reply := proxyManagerRequest(t, client, IsProxyHandlerExistCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	exists, err := reply.ReplyParameters().BoolValue("exists")
	require.NoError(t, err)
	require.Equal(t, expected, exists)
}

func requireProxyHandlerRunning(t *testing.T, client *clientSyncReplier.Client, category string, expected bool) {
	t.Helper()

	reply := proxyManagerRequest(t, client, IsProxyHandlerRunningCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	running, err := reply.ReplyParameters().BoolValue("running")
	require.NoError(t, err)
	require.Equal(t, expected, running)
}

func newProxyHandlerClient(t *testing.T, proxyConfig topologyConfig.ProxyHandler) *clientSyncReplier.Client {
	t.Helper()

	client, err := clientSyncReplier.NewClient(proxyConfig.Endpoint.Id, proxyConfig.Endpoint.Port)
	require.NoError(t, err)
	client.Timeout(time.Second)
	client.Attempt(1)
	return client
}

func requireProxifiedReply(t *testing.T, client *clientSyncReplier.Client, command string) {
	t.Helper()

	reply, err := client.Request(&message.Request{
		Command:    command,
		Parameters: datatype.New(),
	})
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	proxified, err := reply.ReplyParameters().BoolValue("proxified: todo need to go through routers")
	require.NoError(t, err)
	require.True(t, proxified)
}

func requireProxyHandlerRequestTimeout(t *testing.T, proxyConfig topologyConfig.ProxyHandler, command string) {
	t.Helper()

	client := newProxyHandlerClient(t, proxyConfig)
	defer client.Close()

	_, err := client.Request(&message.Request{
		Command:    command,
		Parameters: datatype.New(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "request_timeout")
}
