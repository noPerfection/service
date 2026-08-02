package manager

import (
	"errors"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/service/zap"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

const testHandshakeInterval = 50 * time.Millisecond

func testServiceURL(t *testing.T, serviceName string) mushroom.TopologyURL {
	t.Helper()
	serviceURL, err := mushroom.Parse("*pkg:$?var=services[name:" + serviceName + "]")
	require.NoError(t, err)
	return serviceURL
}

func publishManagerPublicKey(t *testing.T, serviceName, publicKey string) {
	t.Helper()

	client, err := topology.NewClient()
	require.NoError(t, err)
	defer client.Close()

	service, err := client.Service(serviceName)
	require.NoError(t, err)
	if service.Parameters == nil {
		service.Parameters = datatype.New()
	}
	service.Parameters[ManagerPublicKeyParam] = publicKey
	require.NoError(t, client.SetService(service))
}

type fakeCalleeManager struct {
	name     string
	endpoint message.Endpoint
	running  bool
}

func calleeServiceConfig(t *testing.T, name string) topologyConfig.Service {
	t.Helper()
	endpoint := message.NewEndpoint(testEndpointID(t, name+"-manager"), 0)
	return topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      name,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers: []topologyConfig.Handler{
			topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: "main",
					Endpoint: message.NewEndpoint(testEndpointID(t, name+"-main"), 0),
				},
			},
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topologyConfig.ServiceManagerCategory,
				Endpoint: endpoint,
			},
		},
	}
}

func startFakeCalleeManager(t *testing.T, service topologyConfig.Service, allowedCallerPubKey string) *fakeCalleeManager {
	t.Helper()

	managerHandler, err := service.HandlerByCategory(topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	endpoint, ok := managerHandler.AsIndependentHandler()
	require.True(t, ok)

	handler := protocolHandler.NewSyncReplier()
	handler.SetEndpoint(endpoint.Endpoint)
	pub, sec, err := message.GenerateCurveKey()
	require.NoError(t, err)
	handler.Secure(sec)
	mushroomURL, err := mushroom.As("*pkg:$?var=services[name:"+service.Name+"]", topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	handler.SetMushroomURL(mushroomURL.String())
	require.NoError(t, zap.Start())
	if allowedCallerPubKey != "" {
		zap.AuthCurveAdd(mushroomURL.As(mushroom.HANDLER).String(), allowedCallerPubKey)
	}
	for _, cmd := range managerWhitelistCommands() {
		handler.RequireWhitelist(cmd)
	}
	require.NoError(t, handler.Route(Handshake, func(req message.RequestInterface) message.ReplyInterface {
		secret, err := req.RouteParameters().StringValue("manager-hmac-secret")
		if err != nil {
			return req.Fail(err.Error())
		}
		for _, cmd := range managerWhitelistCommands() {
			if err := handler.Whitelist(cmd, secret); err != nil {
				return req.Fail(err.Error())
			}
		}
		return req.Ok(datatype.New().Set("inbounds", map[string]string{}).Set("outbounds", map[string]string{}))
	}))
	callee := &fakeCalleeManager{name: service.Name, endpoint: endpoint.Endpoint}
	require.NoError(t, handler.Route(IsServiceRunning, func(req message.RequestInterface) message.ReplyInterface {
		serviceName, err := req.RouteParameters().StringValue("service")
		if err != nil {
			return req.Fail(err.Error())
		}
		if serviceName != service.Name {
			return req.Fail("unexpected service name")
		}
		return req.Ok(datatype.New().Set("running", callee.running))
	}))
	require.NoError(t, handler.Start())

	publishManagerPublicKey(t, service.Name, pub)

	t.Cleanup(func() {
		_ = handlers.CloseViaControl(handler)
	})
	return callee
}

func lifecycleCallerService(t *testing.T, callerName string) (topologyConfig.Service, message.Endpoint) {
	t.Helper()
	endpoint := message.NewEndpoint(testEndpointID(t, callerName+"-manager"), 0)
	return topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      callerName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topologyConfig.ServiceManagerCategory,
				Endpoint: endpoint,
			},
		},
	}, endpoint
}

func calleeDepURL(t *testing.T, calleeName string) string {
	t.Helper()
	return testServiceURL(t, calleeName).AsDereference().String()
}

func refreshCalleeHandshake(t *testing.T, caller *Manager, calleeName string) {
	t.Helper()
	depURL := calleeDepURL(t, calleeName)
	running, runErr := caller.IsServiceRunning(depURL, 1)
	if runErr != nil && errors.Is(runErr, message.ErrAccessDenied) {
		require.NoError(t, caller.whitelistSelfInDeps(depURL))
		running, runErr = caller.IsServiceRunning(depURL, 1)
	}
	require.NoError(t, runErr)
	_ = running
}

func startLifecycleCallerManager(
	t *testing.T,
	service topologyConfig.Service,
	endpoint message.Endpoint,
	background bool,
) (*Manager, mushroom.TopologyURL) {
	t.Helper()

	serviceURL := testServiceURL(t, service.Name)
	manager, err := New(serviceURL, endpoint)
	require.NoError(t, err)
	if background {
		manager.handshakeInterval = testHandshakeInterval
	} else {
		manager.handshakeInterval = -1
	}
	require.NoError(t, manager.Start())

	t.Cleanup(func() {
		_ = manager.Close()
	})
	return manager, serviceURL
}

func probeCallee(t *testing.T, caller *Manager, calleeName string) (bool, error) {
	t.Helper()
	return caller.IsServiceRunning(testServiceURL(t, calleeName).AsDereference().String())
}

func TestHandshakeLifecycleCalleeStoppedThenStarted(t *testing.T) {
	calleeName := testEndpointID(t, "callee")
	callerName := testEndpointID(t, "caller")

	callerService, callerEndpoint := lifecycleCallerService(t, callerName)
	calleeService := calleeServiceConfig(t, calleeName)
	startTestRuntimeHandler(t, calleeService, callerService)
	caller, callerURL := startLifecycleCallerManager(t, callerService, callerEndpoint, false)

	running, err := probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.False(t, running)

	callee := startFakeCalleeManager(t, calleeService, caller.PublicKey())
	allowManagerClient(t, calleeName, callerURL, caller.PublicKey())
	callee.running = true

	refreshCalleeHandshake(t, caller, calleeName)
	running, err = probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.True(t, running)
}

func TestHandshakeLifecycleReevaluatesWhenCalleeStarts(t *testing.T) {
	calleeName := testEndpointID(t, "callee")
	callerName := testEndpointID(t, "caller")

	callerService, callerEndpoint := lifecycleCallerService(t, callerName)
	calleeService := calleeServiceConfig(t, calleeName)
	startTestRuntimeHandler(t, calleeService, callerService)
	caller, callerURL := startLifecycleCallerManager(t, callerService, callerEndpoint, false)

	callee := startFakeCalleeManager(t, calleeService, caller.PublicKey())
	allowManagerClient(t, calleeName, callerURL, caller.PublicKey())
	refreshCalleeHandshake(t, caller, calleeName)

	callee.running = false
	running, err := probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.False(t, running)

	callee.running = true
	refreshCalleeHandshake(t, caller, calleeName)
	running, err = probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.True(t, running)
}

func TestHandshakeLifecycleCalleeStoppedAndRunsAgain(t *testing.T) {
	calleeName := testEndpointID(t, "callee")
	callerName := testEndpointID(t, "caller")

	callerService, callerEndpoint := lifecycleCallerService(t, callerName)
	calleeService := calleeServiceConfig(t, calleeName)
	startTestRuntimeHandler(t, calleeService, callerService)
	caller, callerURL := startLifecycleCallerManager(t, callerService, callerEndpoint, false)

	callee := startFakeCalleeManager(t, calleeService, caller.PublicKey())
	allowManagerClient(t, calleeName, callerURL, caller.PublicKey())
	callee.running = true
	refreshCalleeHandshake(t, caller, calleeName)
	running, err := probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.True(t, running)

	callee.running = false
	_, err = probeCallee(t, caller, calleeName)
	require.NoError(t, err)

	callee.running = true
	refreshCalleeHandshake(t, caller, calleeName)
	running, err = probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.True(t, running, "caller should re-handshake after callee comes back")
}

func TestBackgroundHandshakeRecoversCallee(t *testing.T) {
	calleeName := testEndpointID(t, "callee")
	callerName := testEndpointID(t, "caller")

	callerService, callerEndpoint := lifecycleCallerService(t, callerName)
	calleeService := calleeServiceConfig(t, calleeName)
	startTestRuntimeHandler(t, calleeService, callerService)
	caller, callerURL := startLifecycleCallerManager(t, callerService, callerEndpoint, true)

	running, err := probeCallee(t, caller, calleeName)
	require.NoError(t, err)
	require.False(t, running)

	callee := startFakeCalleeManager(t, calleeService, caller.PublicKey())
	allowManagerClient(t, calleeName, callerURL, caller.PublicKey())
	callee.running = true

	require.Eventually(t, func() bool {
		refreshCalleeHandshake(t, caller, calleeName)
		running, err := probeCallee(t, caller, calleeName)
		return err == nil && running
	}, 3*time.Second, testHandshakeInterval, "handshake should recover callee")
}

func TestBackgroundHandshakeStopsWithManager(t *testing.T) {
	calleeName := testEndpointID(t, "callee")
	callerName := testEndpointID(t, "caller")

	callerService, callerEndpoint := lifecycleCallerService(t, callerName)
	calleeService := calleeServiceConfig(t, calleeName)
	startTestRuntimeHandler(t, calleeService, callerService)
	caller, callerURL := startLifecycleCallerManager(t, callerService, callerEndpoint, true)

	callee := startFakeCalleeManager(t, calleeService, caller.PublicKey())
	allowManagerClient(t, calleeName, callerURL, caller.PublicKey())
	callee.running = true
	refreshCalleeHandshake(t, caller, calleeName)

	require.Eventually(t, func() bool {
		running, err := probeCallee(t, caller, calleeName)
		return err == nil && running
	}, 3*time.Second, testHandshakeInterval)

	require.NoError(t, caller.StopService(caller.serviceURL.AsDereference().String()))
	require.False(t, caller.Running())
}

func allowManagerClient(t *testing.T, calleeName string, callerServiceURL mushroom.TopologyURL, callerPublicKey string) {
	t.Helper()

	client, err := topology.NewClient()
	require.NoError(t, err)
	defer client.Close()

	service, err := client.Service(calleeName)
	require.NoError(t, err)
	callerManagerLink := callerServiceURL.New(topologyConfig.ServiceManagerCategory)
	mushroom.AddAllowedPublicKey(&service, callerManagerLink, callerServiceURL.ResourcePublicKey())
	if service.Parameters == nil {
		service.Parameters = datatype.New()
	}
	if allowed, ok := service.Parameters["allowed"].(map[string]any); ok {
		if managerAllowed, ok := allowed[topologyConfig.ServiceManagerCategory].(map[string]any); ok {
			managerAllowed[callerManagerLink.String()] = callerPublicKey
		}
	}
	require.NoError(t, client.SetService(service))
}

func TestHandshakeRouteRegisteredOnStart(t *testing.T) {
	callerName := testEndpointID(t, "caller")
	callerService, callerEndpoint := lifecycleCallerService(t, callerName)
	startTestRuntimeHandler(t, callerService)
	caller, callerURL := startLifecycleCallerManager(t, callerService, callerEndpoint, false)

	publishManagerPublicKey(t, callerService.Name, caller.PublicKey())

	managerURL, err := mushroom.As(callerURL.String(), topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)

	msg := &message.Request{
		Command: Handshake,
		Parameters: datatype.New().
			Set("manager-hmac-secret", message.GenerateSecret()).
			Set("manager-url", managerURL.String()).
			Set("in-inbounds", datatype.New()).
			Set("in-outbounds", datatype.New()),
	}
	signature, err := message.Sign(msg.String(), caller.curveSecretKey)
	require.NoError(t, err)
	msg.Parameters.Set("signature", signature)

	client, err := protocolClient.NewSyncReplier(callerEndpoint.Id, callerEndpoint.Port)
	require.NoError(t, err)
	defer client.Close()
	client.Secure(caller.curveSecretKey)

	reply, err := client.Request(msg)
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())
}
