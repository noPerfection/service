package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

var testEndpointSeq atomic.Uint64
var testRuntimeHandler struct {
	once    sync.Once
	handler *topology.Handler
	err     error
}

func testEndpointID(t *testing.T, name string) string {
	t.Helper()
	seq := testEndpointSeq.Add(1)
	return fmt.Sprintf("%s_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), name, seq)
}

func fakeServiceConfig(serviceName string, managerEndpoint message.Endpoint, handlers ...topologyConfig.IndependentHandler) topologyConfig.Service {
	serviceHandlers := []topologyConfig.IndependentHandler{
		{
			Type:     topologyConfig.SyncReplierType,
			Category: topologyConfig.ServiceManagerCategory,
			Endpoint: managerEndpoint,
		},
	}
	serviceHandlers = append(serviceHandlers, handlers...)

	handlerList := make([]topologyConfig.Handler, len(serviceHandlers))
	for i, h := range serviceHandlers {
		handlerList[i] = h
	}

	return topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      serviceName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers:  handlerList,
	}
}

func fakeHandlerConfig(t *testing.T, handlerType topologyConfig.HandlerType, category string) topologyConfig.IndependentHandler {
	t.Helper()
	return topologyConfig.IndependentHandler{
		Type:     handlerType,
		Category: category,
		Endpoint: message.NewEndpoint(testEndpointID(t, category), 0),
	}
}

func startTestRuntimeHandler(t *testing.T, services ...topologyConfig.Service) {
	t.Helper()

	testRuntimeHandler.once.Do(func() {
		dir, err := os.MkdirTemp("", "service-manager-test-*")
		if err != nil {
			testRuntimeHandler.err = err
			return
		}
		appPath := filepath.Join(dir, "app.json")
		appConfig, err := topologyConfig.Load(appPath)
		if err != nil {
			testRuntimeHandler.err = err
			return
		}
		if err := appConfig.Save(); err != nil {
			testRuntimeHandler.err = err
			return
		}

		handler, err := topology.NewHandler(appPath)
		if err != nil {
			testRuntimeHandler.err = err
			return
		}
		if err := handler.Start(); err != nil {
			testRuntimeHandler.err = err
			return
		}
		testRuntimeHandler.handler = handler
	})
	require.NoError(t, testRuntimeHandler.err)
	require.NotNil(t, testRuntimeHandler.handler)

	client, err := topology.NewClient()
	require.NoError(t, err)
	defer client.Close()
	for _, service := range services {
		require.NoError(t, client.AddService(service))
	}
}

func newProtocolHandler(t *testing.T, handlerType topologyConfig.HandlerType) protocolHandler.Interface {
	t.Helper()

	switch handlerType {
	case topologyConfig.SyncReplierType:
		return protocolHandler.NewSyncReplier()
	case topologyConfig.ReplierType:
		return protocolHandler.NewReplier()
	case topologyConfig.PublisherType:
		return protocolHandler.NewPublisher()
	case topologyConfig.WorkerType:
		return protocolHandler.NewWorker()
	default:
		t.Fatalf("unsupported handler type: %s", handlerType)
		return nil
	}
}

func startFakeServiceHandlers(t *testing.T, service topologyConfig.Service) []protocolHandler.Interface {
	t.Helper()

	startedHandlers := make([]protocolHandler.Interface, 0, len(service.Handlers))
	for _, configuredVariant := range service.Handlers {
		configured, ok := configuredVariant.AsIndependentHandler()
		if !ok {
			continue
		}
		if configured.Category == topologyConfig.ServiceManagerCategory {
			continue
		}

		handler := newProtocolHandler(t, configured.Type)
		handler.SetEndpoint(configured.Endpoint)
		mushroomURL, err := mushroom.As("*pkg:$?var=services[name:"+service.Name+"]", configured.Category)
		require.NoError(t, err)
		handler.SetMushroomURL(mushroomURL.String())
		require.NoError(t, handler.Start())
		startedHandlers = append(startedHandlers, handler)
	}

	t.Cleanup(func() {
		for _, handler := range startedHandlers {
			_ = handlers.CloseViaControl(handler)
		}
	})

	return startedHandlers
}

func handlerControlStatus(handler protocolHandler.Interface) (string, error) {
	endpoint := handler.Endpoint()
	if endpoint == (message.Endpoint{}) {
		return "", fmt.Errorf("handler endpoint is empty")
	}
	controlEndpoint := protocolHandler.NewInternalControlEndpoint(endpoint)
	controlClient, err := protocolClient.NewControl(controlEndpoint.Id, controlEndpoint.Port)
	if err != nil {
		return "", fmt.Errorf("sync_replier.NewBaseControl('%s'): %w", controlEndpoint.Id, err)
	}
	controlClient.Timeout(time.Second)
	controlClient.Attempt(3)
	defer controlClient.Close()
	return controlClient.HandlerStatus()
}

func requireHandlersStopped(t *testing.T, started []protocolHandler.Interface) {
	t.Helper()

	require.Eventually(t, func() bool {
		for _, handler := range started {
			status, err := handlerControlStatus(handler)
			if err != nil || status != protocolHandler.SocketNil {
				return false
			}
		}
		return true
	}, 2*time.Second, 10*time.Millisecond)
}

func requireHandlerStopped(t *testing.T, handler protocolHandler.Interface) {
	t.Helper()

	require.Eventually(t, func() bool {
		status, err := handlerControlStatus(handler)
		return err == nil && status == protocolHandler.SocketNil
	}, 2*time.Second, 10*time.Millisecond)
}

func newTestManager(t *testing.T, service topologyConfig.Service, managerEndpoint message.Endpoint) *Manager {
	t.Helper()

	serviceURL, err := mushroom.Parse("*pkg:$?var=services[name:" + service.Name + "]")
	require.NoError(t, err)
	manager, err := New(serviceURL, managerEndpoint)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = manager.Close()
		time.Sleep(20 * time.Millisecond)
	})
	return manager
}

func TestSetHandlerControlsMatchesFakeServiceConfig(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(
		serviceName,
		managerEndpoint,
		fakeHandlerConfig(t, topologyConfig.SyncReplierType, "sync"),
		fakeHandlerConfig(t, topologyConfig.ReplierType, "async"),
		fakeHandlerConfig(t, topologyConfig.PublisherType, "events"),
	)
	handlers := startFakeServiceHandlers(t, service)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)

	require.NoError(t, manager.setHandlerControls())
	require.Len(t, manager.handlerControls, len(handlers))

	for _, handlerVariant := range service.Handlers {
		handler, ok := handlerVariant.AsIndependentHandler()
		if !ok || handler.Category == topologyConfig.ServiceManagerCategory {
			continue
		}

		controlClient, ok := manager.handlerControls[handler.Category]
		require.True(t, ok)

		handlerControlConfig, err := controlClient.HandlerConfig()
		require.NoError(t, err)
		require.Equal(t, handler.Endpoint.Id, handlerControlConfig.Id)
		require.Equal(t, handler.Endpoint.Port, handlerControlConfig.Port)
	}
}

func TestRemoteServicesReturnsConfiguredServices(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(serviceName, managerEndpoint)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	require.NoError(t, manager.Start())

	client, err := protocolClient.NewSyncReplier(managerEndpoint.Id, managerEndpoint.Port)
	require.NoError(t, err)
	defer client.Close()

	reply, err := client.Request(&message.Request{
		Command:    Services,
		Parameters: datatype.New(),
	})
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	rawServices, err := reply.ReplyParameters().NestedListValue("services")
	require.NoError(t, err)

	services := make([]topologyConfig.Service, 0, len(rawServices))
	for _, rawService := range rawServices {
		var service topologyConfig.Service
		require.NoError(t, rawService.Interface(&service))
		services = append(services, service)
	}

	serviceNames := make([]string, 0, len(services))
	for _, service := range services {
		serviceNames = append(serviceNames, service.Name)
	}
	require.Contains(t, serviceNames, serviceName)
}

func TestStopServiceWithNilBlockerStopsConfiguredHandlers(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(
		serviceName,
		managerEndpoint,
		fakeHandlerConfig(t, topologyConfig.SyncReplierType, "sync"),
		fakeHandlerConfig(t, topologyConfig.ReplierType, "async"),
		fakeHandlerConfig(t, topologyConfig.PublisherType, "events"),
		fakeHandlerConfig(t, topologyConfig.WorkerType, "jobs"),
	)
	handlers := startFakeServiceHandlers(t, service)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	require.NoError(t, manager.Start())
	require.True(t, manager.Running())
	require.Len(t, manager.handlerControls, len(handlers))

	require.NoError(t, manager.StopService(""))

	require.False(t, manager.Running())
	require.Empty(t, manager.handlerControls)
	requireHandlersStopped(t, handlers)
}

func TestStopServiceWithNilSharedBlockerPointer(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(
		serviceName,
		managerEndpoint,
		fakeHandlerConfig(t, topologyConfig.SyncReplierType, "sync"),
	)
	handlers := startFakeServiceHandlers(t, service)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	var blocker *sync.WaitGroup
	manager.SetSharedBlocker(&blocker)
	require.NoError(t, manager.Start())

	require.NoError(t, manager.StopService(service.Name))

	require.False(t, manager.Running())
	requireHandlersStopped(t, handlers)
}

func TestRemoteStopServiceWithNilBlocker(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(
		serviceName,
		managerEndpoint,
		fakeHandlerConfig(t, topologyConfig.SyncReplierType, "sync"),
	)
	handlers := startFakeServiceHandlers(t, service)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	require.NoError(t, manager.Start())

	client, err := protocolClient.NewSyncReplier(managerEndpoint.Id, managerEndpoint.Port)
	require.NoError(t, err)
	defer client.Close()

	reply, err := client.Request(&message.Request{
		Command:    StopService,
		Parameters: datatype.New().Set("service", service.Name),
	})
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())

	require.Eventually(t, func() bool {
		return !manager.Running()
	}, time.Second, 10*time.Millisecond)
	requireHandlersStopped(t, handlers)
}

func TestStopServiceReleasesBlockerOnce(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(
		serviceName,
		managerEndpoint,
		fakeHandlerConfig(t, topologyConfig.SyncReplierType, "sync"),
	)
	startFakeServiceHandlers(t, service)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	blocker := &sync.WaitGroup{}
	blocker.Add(1)
	sharedBlocker := blocker
	manager.SetSharedBlocker(&sharedBlocker)
	require.NoError(t, manager.Start())

	released := make(chan struct{})
	go func() {
		blocker.Wait()
		close(released)
	}()

	require.NoError(t, manager.StopService(service.Name))
	require.Eventually(t, func() bool {
		select {
		case <-released:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, manager.StopService(service.Name))
}

func TestCloseStopsConfiguredHandlersAndManagerSockets(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(
		serviceName,
		managerEndpoint,
		fakeHandlerConfig(t, topologyConfig.SyncReplierType, "sync"),
		fakeHandlerConfig(t, topologyConfig.ReplierType, "async"),
	)
	handlers := startFakeServiceHandlers(t, service)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	require.NoError(t, manager.Start())

	require.NoError(t, manager.Close())

	require.False(t, manager.Running())
	require.Empty(t, manager.handlerControls)
	requireHandlerStopped(t, manager.Interface)
	requireHandlersStopped(t, handlers)
}

func TestStartFailsWhenTopologyClientIsNil(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceURL, err := mushroom.Parse("*pkg:$?var=services[name:fake-service]")
	require.NoError(t, err)
	manager, err := New(serviceURL, managerEndpoint)
	require.NoError(t, err)
	manager.topology = nil

	require.EqualError(t, manager.Start(), "setHandlerControls: topology is nil")
	require.False(t, manager.Running())
}

func TestServiceNameValidation(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceURL, err := mushroom.Parse("*pkg:$?var=services[name:fake-service]")
	require.NoError(t, err)
	manager, err := New(serviceURL, managerEndpoint)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = manager.Close()
	})

	manager.topology = nil

	_, err = manager.StartService("other-service")
	require.EqualError(t, err, "topology is nil")

	_, err = manager.IsServiceRunning("other-service")
	require.EqualError(t, err, "topology is nil")

	require.EqualError(t, manager.StopService("other-service"), "topology is nil")
}

type recordingInprocTopologyExtension struct {
	started map[string]bool
}

func newRecordingInprocTopologyExtension() *recordingInprocTopologyExtension {
	return &recordingInprocTopologyExtension{
		started: make(map[string]bool),
	}
}

func startRecordingInprocTopologyExtension(t *testing.T, endpoint message.Endpoint) *recordingInprocTopologyExtension {
	t.Helper()

	recorder := newRecordingInprocTopologyExtension()
	handler := protocolHandler.NewReplier()
	handler.SetEndpoint(endpoint)
	require.NoError(t, handler.Route(StartService, func(req message.RequestInterface) message.ReplyInterface {
		serviceName, err := req.RouteParameters().StringValue("service")
		if err != nil {
			return req.Fail(err.Error())
		}
		recorder.started[serviceName] = true
		return req.Ok(datatype.New().Set("id", "1"))
	}))
	mushroomURL, err := mushroom.As("*pkg:$?var=services[name:inproc-topology]", handlers.DefaultHandlerCategory)
	require.NoError(t, err)
	handler.SetMushroomURL(mushroomURL.String())
	require.NoError(t, handler.Start())
	t.Cleanup(func() {
		_ = handlers.CloseViaControl(handler)
	})
	return recorder
}

type recordingServiceManager struct {
	stopped map[string]bool
	probe   map[string]bool
}

func startRecordingServiceManager(t *testing.T, endpoint message.Endpoint) *recordingServiceManager {
	t.Helper()

	recorder := &recordingServiceManager{
		stopped: make(map[string]bool),
		probe:   make(map[string]bool),
	}
	handler := protocolHandler.NewSyncReplier()
	handler.SetEndpoint(endpoint)
	require.NoError(t, handler.Route(IsServiceRunning, func(req message.RequestInterface) message.ReplyInterface {
		serviceName, err := req.RouteParameters().StringValue("service")
		if err != nil {
			return req.Fail(err.Error())
		}
		return req.Ok(datatype.New().Set("running", recorder.probe[serviceName]))
	}))
	require.NoError(t, handler.Route(StopService, func(req message.RequestInterface) message.ReplyInterface {
		serviceName, err := req.RouteParameters().StringValue("service")
		if err != nil {
			return req.Fail(err.Error())
		}
		recorder.stopped[serviceName] = true
		recorder.probe[serviceName] = false
		return req.Ok(datatype.New())
	}))
	mushroomURL, err := mushroom.As("*pkg:$?var=services[name:test-service]", topologyConfig.ServiceManagerCategory)
	require.NoError(t, err)
	handler.SetMushroomURL(mushroomURL.String())
	require.NoError(t, handler.Start())
	t.Cleanup(func() {
		_ = handlers.CloseViaControl(handler)
	})
	return recorder
}

func TestManagerDelegatesInprocStartStop(t *testing.T) {
	inprocTopologyExtension := message.NewEndpoint(testEndpointID(t, "inproc-topology-extension"), 0)
	inprocRecorder := startRecordingInprocTopologyExtension(t, inprocTopologyExtension)
	inprocTopologyManager := message.NewEndpoint(testEndpointID(t, "inproc-topology-manager"), 0)
	inprocTopology := topologyConfig.Service{
		Type:      topologyConfig.ExtensionType,
		Name:      InprocTopologyServiceName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topologyConfig.ServiceManagerCategory,
				Endpoint: inprocTopologyManager,
			},
			topologyConfig.ExtensionHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.ReplierType,
					Category: handlers.DefaultHandlerCategory,
					Endpoint: inprocTopologyExtension,
				},
			},
		},
	}

	hostManager := message.NewEndpoint(testEndpointID(t, "host-manager"), 0)
	host := fakeServiceConfig("host", hostManager)

	childManager := message.NewEndpoint(testEndpointID(t, "child-manager"), 0)
	childRecorder := startRecordingServiceManager(t, childManager)
	childRecorder.probe["child"] = true

	child := topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      "child",
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers: []topologyConfig.Handler{
			topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: "main",
					Endpoint: message.NewEndpoint(testEndpointID(t, "child"), 0),
				},
			},
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topologyConfig.ServiceManagerCategory,
				Endpoint: childManager,
			},
		},
	}
	startTestRuntimeHandler(t, host, inprocTopology, child)

	manager := newTestManager(t, host, hostManager)
	id, err := manager.StartService("child")
	require.NoError(t, err)
	require.Equal(t, "1", id)
	require.True(t, inprocRecorder.started["child"])

	running, err := manager.IsServiceRunning("child")
	require.NoError(t, err)
	require.True(t, running)

	require.NoError(t, manager.StopService("child"))
	require.True(t, childRecorder.stopped["child"])
}
