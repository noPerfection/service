package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/control"
	handlerPublisher "github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

var testTopologyRuntime struct {
	once    sync.Once
	handler *topology.Handler
	err     error
}

func testOutboundURL(serviceName, category string) string {
	return fmt.Sprintf("pkg:$?var=services[name:%s]&category=%s", serviceName, category)
}

func parseTestOutboundURL(url string) (serviceName, category string, err error) {
	const namePrefix = "name:"
	nameStart := strings.Index(url, namePrefix)
	if nameStart < 0 {
		return "", "", fmt.Errorf("outbound url %q has no service name", url)
	}
	nameStart += len(namePrefix)
	nameEnd := strings.Index(url[nameStart:], "]")
	if nameEnd < 0 {
		return "", "", fmt.Errorf("outbound url %q has invalid service name", url)
	}
	serviceName = url[nameStart : nameStart+nameEnd]

	const categoryPrefix = "&category="
	categoryStart := strings.Index(url, categoryPrefix)
	if categoryStart < 0 {
		return "", "", fmt.Errorf("outbound url %q has no handler category", url)
	}
	category = url[categoryStart+len(categoryPrefix):]
	if cut, _, ok := strings.Cut(category, "&"); ok {
		category = cut
	}
	if category == "" {
		return "", "", fmt.Errorf("outbound url %q has empty handler category", url)
	}
	return serviceName, category, nil
}

func startTestTopologyHandler(t *testing.T) {
	t.Helper()

	testTopologyRuntime.once.Do(func() {
		dir, err := os.MkdirTemp("", "handlers-proxy-test-*")
		if err != nil {
			testTopologyRuntime.err = err
			return
		}
		appPath := filepath.Join(dir, "app.json")
		appConfig, err := topologyConfig.Load(appPath)
		if err != nil {
			testTopologyRuntime.err = err
			return
		}
		if err := appConfig.Save(); err != nil {
			testTopologyRuntime.err = err
			return
		}

		handler, err := topology.NewHandler(appPath)
		if err != nil {
			testTopologyRuntime.err = err
			return
		}
		if err := handler.Start(); err != nil {
			testTopologyRuntime.err = err
			return
		}
		testTopologyRuntime.handler = handler
	})
	require.NoError(t, testTopologyRuntime.err)
	require.NotNil(t, testTopologyRuntime.handler)
}

func addTestTopologyServices(t *testing.T, services ...topologyConfig.Service) {
	t.Helper()
	startTestTopologyHandler(t)

	client, err := topology.NewClient()
	require.NoError(t, err)
	defer client.Close()
	for _, service := range services {
		require.NoError(t, client.AddService(service))
	}
}

func outboundService(serviceName string, handlers ...topologyConfig.IndependentHandler) topologyConfig.Service {
	handlerList := make([]topologyConfig.Handler, len(handlers))
	for i, handler := range handlers {
		handlerList[i] = handler
	}
	return topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      serviceName,
		ModuleUrl: "github.com/noPerfection/service/handlers/test",
		Handlers:  handlerList,
	}
}

func registerProxyHandlerOutbounds(t *testing.T, proxyConfig topologyConfig.ProxyHandler) {
	t.Helper()

	byService := make(map[string][]topologyConfig.IndependentHandler)
	for _, url := range proxyConfig.Outbounds {
		serviceName, category, err := parseTestOutboundURL(url)
		require.NoError(t, err)
		byService[serviceName] = append(byService[serviceName], topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: category,
			Endpoint: message.NewEndpoint(testEndpointID(t, "outbound-"+serviceName+"-"+category), 0),
		})
	}

	services := make([]topologyConfig.Service, 0, len(byService))
	for serviceName, handlers := range byService {
		services = append(services, outboundService(serviceName, handlers...))
	}
	addTestTopologyServices(t, services...)
}

func registerOutboundHandlers(t *testing.T, serviceName string, handlers ...topologyConfig.IndependentHandler) {
	t.Helper()
	addTestTopologyServices(t, outboundService(serviceName, handlers...))
}

func handlersOf(handlers ...topologyConfig.IndependentHandler) []topologyConfig.Handler {
	result := make([]topologyConfig.Handler, len(handlers))
	for i, h := range handlers {
		result[i] = h
	}
	return result
}

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

func TestValidateProxyHandlerOutboundsRequiresURL(t *testing.T) {
	tests := []struct {
		name        string
		outbounds   []string
		expectedErr string
	}{
		{
			name:        "empty",
			outbounds:   nil,
			expectedErr: "not possible to send since no outbound yet",
		},
		{
			name:        "missing url",
			outbounds:   []string{""},
			expectedErr: "outbounds[0] url is required",
		},
		{
			name:      "valid url",
			outbounds: []string{testOutboundURL("api", "main")},
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
	registerProxyHandlerOutbounds(t, proxyConfig)
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
	noOutbounds.Outbounds = []string{}
	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, proxyManagerConfigParams(t, noOutbounds))
	require.False(t, reply.IsOK())
	require.Equal(t, "not possible to send since no outbound yet", reply.ErrorMessage())

	noProxyHandle := validProxyHandlerConfig(t, "without-proxy-handle")
	noProxyHandle.Routes = nil
	reply = proxyManagerRequest(t, client, SetProxyHandlerCommand, proxyManagerConfigParams(t, noProxyHandle))
	require.False(t, reply.IsOK())
	require.Equal(t, "can not set a proxy since no proxy handle for `without-proxy-handle` or `default` for any command proxy handle is set", reply.ErrorMessage())

	reply = proxyManagerRequest(t, client, IsProxyHandlerRunningCommand, proxyManagerCategoryParams(category))
	require.False(t, reply.IsOK())
	require.Equal(t, "No proxified handler was set, please call set-proxy-handler-command command to set it first", reply.ErrorMessage())
	requireProxyHandlerExists(t, client, category, false)

	withConfigRoutes := proxyConfig
	withConfigRoutes.Routes = nil
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

func TestProxyHandlersRouteAnyDefaultAndCategoryRules(t *testing.T) {
	t.Run("no category requires user handler and sets manager routes", func(t *testing.T) {
		manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-route-default"))

		require.EqualError(t, manager.Route(base.Any, nil), "proxy handle function is required when command is '*'")
		require.NoError(t, manager.Route(base.Any, proxyOKRoute))
		require.NotNil(t, manager.routes[base.Any])
		require.NoError(t, manager.Route("hello", proxyOKRoute))
		require.NotNil(t, manager.routes["hello"])
		require.Empty(t, manager.proxifiedHandlers)
	})

	t.Run("any with category requires user handler and sets category route only", func(t *testing.T) {
		manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-route-category-default"))

		require.EqualError(t, manager.Route(base.Any, nil, "api"), "proxy handle function is required when command is '*'")
		require.NoError(t, manager.Route(base.Any, proxyOKRoute, "api"))
		require.Empty(t, manager.routes)
		require.NotNil(t, manager.proxifiedHandlers[Category("api")].routes[base.Any])
	})

	t.Run("named command with category requires handler", func(t *testing.T) {
		manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-route-named"))

		require.EqualError(t, manager.Route("hello", nil, "api"), "proxy handle function is required when command is 'hello'")

		require.NoError(t, manager.Route("hello", proxyOKRoute, "api"))
		require.NotNil(t, manager.proxifiedHandlers[Category("api")].routes["hello"])
	})
}

func TestProxyHandlersStartStopProxyHandler(t *testing.T) {
	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-start-stop"))
	category := "api"
	proxyConfig := validProxyHandlerConfig(t, category)
	registerProxyHandlerOutbounds(t, proxyConfig)
	proxyConfig.Routes = []string{base.Any}
	require.NoError(t, manager.Route("hello", proxyOKRoute, category))
	require.NoError(t, manager.Route(base.Any, proxyOKRoute, category))

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

func TestProxyHandlersHandleFuncWhitelistAndRouteFallback(t *testing.T) {
	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-handle-func"))
	require.NoError(t, manager.Route(base.Any, proxyMessageRoute("Whitelisted default")))
	require.NoError(t, manager.Route(base.Any, proxyMessageRoute("handler's default any is returned"), "handler-any"))
	require.NoError(t, manager.Route("hello", proxyMessageRoute("hello from manager")))

	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		_ = manager.Close()
	})

	managerClient, err := clientSyncReplier.NewClient(manager.Interface.Config().Id, manager.Interface.Config().Port)
	require.NoError(t, err)
	managerClient.Timeout(time.Second)
	managerClient.Attempt(3)
	defer managerClient.Close()

	defaultConfig := validProxyHandlerConfig(t, "default-whitelist")
	defaultConfig.Routes = nil
	registerProxyHandlerOutbounds(t, defaultConfig)
	requireStartedProxyConfig(t, managerClient, defaultConfig)
	defaultClient := newProxyHandlerClient(t, defaultConfig)
	requireProxyMessage(t, defaultClient, "anything", "Whitelisted default")
	require.NoError(t, defaultClient.Close())
	requireStoppedAndRemovedProxyConfig(t, managerClient, defaultConfig.Category)

	handlerAnyConfig := validProxyHandlerConfig(t, "handler-any")
	handlerAnyConfig.Routes = []string{base.Any}
	registerProxyHandlerOutbounds(t, handlerAnyConfig)
	requireStartedProxyConfig(t, managerClient, handlerAnyConfig)
	handlerAnyClient := newProxyHandlerClient(t, handlerAnyConfig)
	requireProxyMessage(t, handlerAnyClient, "something-random", "handler's default any is returned")
	require.NoError(t, handlerAnyClient.Close())
	requireStoppedAndRemovedProxyConfig(t, managerClient, handlerAnyConfig.Category)

	delete(manager.routes, base.Any)

	managerHelloConfig := validProxyHandlerConfig(t, "manager-hello")
	managerHelloConfig.Routes = nil
	registerProxyHandlerOutbounds(t, managerHelloConfig)
	requireStartedProxyConfig(t, managerClient, managerHelloConfig)
	managerHelloClient := newProxyHandlerClient(t, managerHelloConfig)
	requireProxyFailure(t, managerHelloClient, "bye", "can not find the proxy handler")
	requireProxyMessage(t, managerHelloClient, "hello", "hello from manager")
	require.NoError(t, managerHelloClient.Close())
	requireStoppedAndRemovedProxyConfig(t, managerClient, managerHelloConfig.Category)

	manager.routes[base.Any] = proxyMessageRoute("manager any")

	managerAnyConfig := validProxyHandlerConfig(t, "manager-any")
	managerAnyConfig.Routes = nil
	registerProxyHandlerOutbounds(t, managerAnyConfig)
	requireStartedProxyConfig(t, managerClient, managerAnyConfig)
	managerAnyClient := newProxyHandlerClient(t, managerAnyConfig)
	requireProxyMessage(t, managerAnyClient, "whatever", "manager any")
	require.NoError(t, managerAnyClient.Close())
}

func TestProxyHandlersSerializeDeserializeRequestOutbound(t *testing.T) {
	emptyManager := NewProxyHandlers(testEndpointID(t, "proxy-manager-empty-outbound"))
	request := &message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	}
	_, err := emptyManager.DeserializeRequest(message.MessageToEnvelope("", request.String()))
	require.EqualError(t, err, "no proxified handlers")
	_, err = emptyManager.DeserializeRequest(message.MessageToEnvelope("", request.String(), "pkg:$?var=services[name:missing]&category=main"))
	require.EqualError(t, err, `outbound "pkg:$?var=services[name:missing]&category=main" not found`)

	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-outbound"))
	proxyConfig := validProxyHandlerConfig(t, "api")
	manager.proxifiedHandlers[Category(proxyConfig.Category)] = &ProxifiedHandler{
		routes:      make(map[string]ProxyHandleFunc),
		proxyConfig: proxyConfig,
	}

	raw, err := manager.DeserializeRequest(message.MessageToEnvelope("", request.String()))
	require.NoError(t, err)
	proxyRequest := raw.(*ProxyRequest)
	require.Equal(t, "api", proxyRequest.proxifiedHandler)
	require.Equal(t, proxyConfig.Outbounds[0], proxyRequest.outboundURL)

	envelope, err := manager.SerializeRequest(proxyRequest)
	require.NoError(t, err)
	require.Equal(t, []string{"", request.String(), proxyConfig.Outbounds[0]}, envelope)

	raw, err = manager.DeserializeRequest(message.MessageToEnvelope("", request.String(), proxyConfig.Outbounds[0]))
	require.NoError(t, err)
	proxyRequest = raw.(*ProxyRequest)
	require.Equal(t, "api", proxyRequest.proxifiedHandler)
	require.Equal(t, proxyConfig.Outbounds[0], proxyRequest.outboundURL)

	_, err = manager.DeserializeRequest(message.MessageToEnvelope("", request.String(), "missing-service"))
	require.EqualError(t, err, `outbound "missing-service" not found`)
}

func TestProxyRequestForwardUsesOutboundClients(t *testing.T) {
	serviceName := "outbound-forward"
	outboundHandlers := []topologyConfig.IndependentHandler{
		startForwardOutboundHandler(t, handlerConfig.SyncReplierType, "sync", "sync reply"),
		startForwardOutboundHandler(t, handlerConfig.ReplierType, "replier", "replier reply"),
		startForwardOutboundHandler(t, handlerConfig.PairType, "pair", "pair reply"),
		startForwardOutboundHandler(t, handlerConfig.WorkerType, "worker", "worker reply"),
		startForwardPublisher(t, serviceName, "publisher", "publisher reply"),
	}

	outboundURLs := make([]string, 0, len(outboundHandlers))
	for _, handler := range outboundHandlers {
		outboundURLs = append(outboundURLs, testOutboundURL(serviceName, handler.Category))
	}
	registerOutboundHandlers(t, serviceName, outboundHandlers...)

	proxyConfig := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: "proxy",
			Endpoint: message.NewEndpoint(testEndpointID(t, "proxy-forward"), 0),
		},
		Outbounds: outboundURLs,
	}

	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-forward"))
	outboundClients, err := manager.newOutboundClients(proxyConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closeOutboundClients(outboundClients))
	})
	startOutboundSubscribers(outboundClients)
	time.Sleep(50 * time.Millisecond)

	manager.proxifiedHandlers[Category(proxyConfig.Category)] = &ProxifiedHandler{
		proxyConfig:     proxyConfig,
		outboundClients: outboundClients,
	}

	cases := []struct {
		category string
		expected string
	}{
		{category: "sync", expected: "sync reply"},
		{category: "replier", expected: "replier reply"},
		{category: "publisher", expected: "publisher reply"},
	}
	for _, tc := range cases {
		t.Run(tc.category, func(t *testing.T) {
			reply, err := proxyForwardRequest(manager, proxyConfig.Category, serviceName, tc.category).Forward()
			require.NoError(t, err)
			require.True(t, reply.IsOK(), reply.ErrorMessage())
			actual, err := reply.ReplyParameters().StringValue("message")
			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}

	reply, err := proxyForwardRequest(manager, proxyConfig.Category, serviceName, "pair").Forward()
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	reply, err = proxyForwardRequest(manager, proxyConfig.Category, serviceName, "worker").Forward()
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	_, err = proxyForwardRequest(manager, proxyConfig.Category, serviceName, "missing").Forward()
	require.EqualError(t, err, `unsupported outbound client for "pkg:$?var=services[name:outbound-forward]&category=missing"`)
}

func TestProxyHandlerRouteForwardsToOutboundAcrossLifecycle(t *testing.T) {
	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-route-forward"))
	proxyCategory := "proxy-forward-route"
	serviceName := "outbound-route-forward"
	outboundHandler := startEchoOutboundHandler(t, "echo")
	registerOutboundHandlers(t, serviceName, outboundHandler)
	proxyConfig := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: proxyCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "proxy-route-forward"), 0),
		},
		Routes:    []string{base.Any},
		Outbounds: []string{testOutboundURL(serviceName, "echo")},
	}

	require.NoError(t, manager.Route(base.Any, proxyForwardRoute, proxyCategory))
	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		_ = manager.Close()
	})

	managerClient, err := clientSyncReplier.NewClient(manager.Interface.Config().Id, manager.Interface.Config().Port)
	require.NoError(t, err)
	managerClient.Timeout(time.Second)
	managerClient.Attempt(3)
	defer managerClient.Close()

	reply := proxyManagerRequest(t, managerClient, SetProxyHandlerCommand, proxyManagerConfigParams(t, proxyConfig))
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	requireProxyHandlerRequestTimeout(t, proxyConfig, "echo")

	reply = proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(proxyCategory))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	proxyClient := newProxyHandlerClient(t, proxyConfig)
	requireForwardedEcho(t, proxyClient, "first")
	require.NoError(t, proxyClient.Close())

	reply = proxyManagerRequest(t, managerClient, StopProxyHandlerCommand, proxyManagerCategoryParams(proxyCategory))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	requireProxyHandlerRequestTimeout(t, proxyConfig, "echo")

	reply = proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(proxyCategory))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	afterRestartClient := newProxyHandlerClient(t, proxyConfig)
	requireForwardedEcho(t, afterRestartClient, "after-restart")
	require.NoError(t, afterRestartClient.Close())
}

func TestProxyHandlerConfiguredForwardOverridesTailOutbound(t *testing.T) {
	manager := NewProxyHandlers(testEndpointID(t, "proxy-manager-configured-forward"))
	proxyCategory := "proxy-forward-config"
	serviceName := "outbound-forward-config"
	defaultHandler := startForwardOutboundHandler(t, handlerConfig.SyncReplierType, "default", "default reply")
	configuredHandler := startForwardOutboundHandler(t, handlerConfig.SyncReplierType, DefaultHandlerCategory, "configured reply")
	registerOutboundHandlers(t, serviceName, defaultHandler, configuredHandler)
	defaultURL := testOutboundURL(serviceName, "default")
	configuredURL := testOutboundURL(serviceName, DefaultHandlerCategory)
	proxyConfig := topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: proxyCategory,
			Endpoint: message.NewEndpoint(testEndpointID(t, "proxy-forward-config"), 0),
		},
		Routes:    []string{"forward"},
		Forward:   map[string]string{"forward": configuredURL},
		Outbounds: []string{defaultURL, configuredURL},
	}

	require.NoError(t, manager.Route(base.Any, proxyForwardRoute, proxyCategory))
	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		_ = manager.Close()
	})

	managerClient, err := clientSyncReplier.NewClient(manager.Interface.Config().Id, manager.Interface.Config().Port)
	require.NoError(t, err)
	managerClient.Timeout(time.Second)
	managerClient.Attempt(3)
	defer managerClient.Close()

	reply := proxyManagerRequest(t, managerClient, SetProxyHandlerCommand, proxyManagerConfigParams(t, proxyConfig))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	reply = proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(proxyCategory))
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	request := &message.Request{
		Command:    "forward",
		Parameters: datatype.New(),
	}
	rawRequest, err := manager.DeserializeRequest(message.MessageToEnvelope("", request.String(), defaultURL))
	require.NoError(t, err)
	reply = manager.handleFunc(rawRequest)
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	message, err := reply.ReplyParameters().StringValue("message")
	require.NoError(t, err)
	require.Equal(t, "configured reply", message)
}

func proxyOKRoute(req ProxyRequest) ProxyReply {
	return ProxyReply{Reply: *req.Ok(datatype.New().Set("proxified: todo need to go through routers", true)).(*message.Reply)}
}

func proxyForwardRoute(req ProxyRequest) ProxyReply {
	reply, err := req.Forward()
	if err != nil {
		return ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}
	return reply
}

func proxyMessageRoute(text string) ProxyHandleFunc {
	return func(req ProxyRequest) ProxyReply {
		return ProxyReply{Reply: *req.Ok(datatype.New().Set("message", text)).(*message.Reply)}
	}
}

func startForwardOutboundHandler(t *testing.T, handlerType handlerConfig.HandlerType, category string, replyText string) topologyConfig.IndependentHandler {
	t.Helper()

	handler := newProtocolHandler(t, handlerType)
	handler.SetConfig(inprocHandlerConfig(handlerType, category, testEndpointID(t, category)))
	require.NoError(t, handler.Route("forward", func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New().Set("message", replyText))
	}))
	require.NoError(t, handler.Start())
	t.Cleanup(func() {
		_ = closeHandlers([]base.Interface{handler})
	})

	return topologyConfig.IndependentHandler{
		Type:     topologyConfig.HandlerType(handlerType),
		Category: category,
		Endpoint: message.NewEndpoint(handler.Config().Id, handler.Config().Port),
	}
}

func startEchoOutboundHandler(t *testing.T, category string) topologyConfig.IndependentHandler {
	t.Helper()

	handler := newProtocolHandler(t, handlerConfig.SyncReplierType)
	handler.SetConfig(inprocHandlerConfig(handlerConfig.SyncReplierType, category, testEndpointID(t, category)))
	require.NoError(t, handler.Route("echo", func(req message.RequestInterface) message.ReplyInterface {
		payload, err := req.RouteParameters().StringValue("payload")
		if err != nil {
			return req.Fail(err.Error())
		}
		return req.Ok(datatype.New().Set("payload", payload))
	}))
	require.NoError(t, handler.Start())
	t.Cleanup(func() {
		_ = closeHandlers([]base.Interface{handler})
	})

	return topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: category,
		Endpoint: message.NewEndpoint(handler.Config().Id, handler.Config().Port),
	}
}

func startForwardPublisher(t *testing.T, serviceName string, category string, replyText string) topologyConfig.IndependentHandler {
	t.Helper()

	handler := newProtocolHandler(t, handlerConfig.PublisherType)
	handler.SetConfig(inprocHandlerConfig(handlerConfig.PublisherType, category, testEndpointID(t, category)))
	require.NoError(t, handler.Start())
	t.Cleanup(func() {
		_ = closeHandlers([]base.Interface{handler})
	})

	controlClient, err := clientSyncReplier.NewClient(control.ControlEndpointID(handler.Config().Id, handler.Config().Port), 0)
	require.NoError(t, err)
	controlClient.Timeout(time.Second)
	controlClient.Attempt(3)
	t.Cleanup(func() {
		_ = controlClient.Close()
	})

	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = controlClient.Request(&message.Request{
			Command: handlerPublisher.Broadcast,
			Parameters: datatype.New().Set(handlerPublisher.BroadcastParameter, message.Reply{
				Status:     message.OK,
				Parameters: datatype.New().Set("message", replyText).Set("service", serviceName),
			}),
		})
	}()

	return topologyConfig.IndependentHandler{
		Type:     topologyConfig.PublisherType,
		Category: category,
		Endpoint: message.NewEndpoint(handler.Config().Id, handler.Config().Port),
	}
}

func proxyForwardRequest(manager *ProxyHandlers, proxifiedCategory string, serviceName string, handlerCategory string) *ProxyRequest {
	return &ProxyRequest{
		Request: message.Request{
			Command:    "forward",
			Parameters: datatype.New(),
		},
		proxifiedHandler: proxifiedCategory,
		outboundURL:      testOutboundURL(serviceName, handlerCategory),
		manager:          manager,
	}
}

func validProxyHandlerConfig(t *testing.T, category string) topologyConfig.ProxyHandler {
	t.Helper()

	outboundName := testEndpointID(t, "outbound-"+category)
	return topologyConfig.ProxyHandler{
		IndependentHandler: topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: category,
			Endpoint: message.NewEndpoint(testEndpointID(t, category), 0),
		},
		Routes:    []string{"hello"},
		Outbounds: []string{testOutboundURL(outboundName, DefaultHandlerCategory)},
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

func requireStartedProxyConfig(t *testing.T, managerClient *clientSyncReplier.Client, proxyConfig topologyConfig.ProxyHandler) {
	t.Helper()

	reply := proxyManagerRequest(t, managerClient, SetProxyHandlerCommand, proxyManagerConfigParams(t, proxyConfig))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	reply = proxyManagerRequest(t, managerClient, StartProxyHandlerCommand, proxyManagerCategoryParams(proxyConfig.Category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
}

func requireStoppedAndRemovedProxyConfig(t *testing.T, managerClient *clientSyncReplier.Client, category string) {
	t.Helper()

	reply := proxyManagerRequest(t, managerClient, StopProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	reply = proxyManagerRequest(t, managerClient, RemoveProxyHandlerCommand, proxyManagerCategoryParams(category))
	require.True(t, reply.IsOK(), reply.ErrorMessage())
}

func requireProxyMessage(t *testing.T, client *clientSyncReplier.Client, command string, expected string) {
	t.Helper()

	reply, err := client.Request(&message.Request{
		Command:    command,
		Parameters: datatype.New(),
	})
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	actual, err := reply.ReplyParameters().StringValue("message")
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func requireForwardedEcho(t *testing.T, client *clientSyncReplier.Client, payload string) {
	t.Helper()

	reply, err := client.Request(&message.Request{
		Command:    "echo",
		Parameters: datatype.New().Set("payload", payload),
	})
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	actual, err := reply.ReplyParameters().StringValue("payload")
	require.NoError(t, err)
	require.Equal(t, payload, actual)
}

func requireProxyFailure(t *testing.T, client *clientSyncReplier.Client, command string, expected string) {
	t.Helper()

	reply, err := client.Request(&message.Request{
		Command:    command,
		Parameters: datatype.New(),
	})
	require.NoError(t, err)
	require.False(t, reply.IsOK())
	require.Equal(t, expected, reply.ErrorMessage())
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
