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
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/handler/worker"
	"github.com/noPerfection/protocol/message"
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

func fakeServiceConfig(serviceName string, managerEndpoint message.Endpoint, handlers ...topologyConfig.Handler) topologyConfig.Service {
	serviceHandlers := []topologyConfig.Handler{
		{
			Type:     topologyConfig.SyncReplierType,
			Category: topology.ServiceManagerCategory,
			Endpoint: managerEndpoint,
		},
	}
	serviceHandlers = append(serviceHandlers, handlers...)

	return topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      serviceName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers:  topologyConfig.NewHandlerVariants(serviceHandlers...),
	}
}

func fakeHandlerConfig(t *testing.T, handlerType topologyConfig.HandlerType, category string) topologyConfig.Handler {
	t.Helper()
	return topologyConfig.Handler{
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

func newProtocolHandler(t *testing.T, handlerType topologyConfig.HandlerType) base.Interface {
	t.Helper()

	switch handlerType {
	case topologyConfig.SyncReplierType:
		return sync_replier.New()
	case topologyConfig.ReplierType:
		return replier.New()
	case topologyConfig.PublisherType:
		return publisher.New()
	case topologyConfig.WorkerType:
		return worker.New()
	default:
		t.Fatalf("unsupported handler type: %s", handlerType)
		return nil
	}
}

func startFakeServiceHandlers(t *testing.T, service topologyConfig.Service) []base.Interface {
	t.Helper()

	handlers := make([]base.Interface, 0, len(service.Handlers))
	for _, configuredVariant := range service.Handlers {
		configured := configuredVariant.AsHandler()
		if configured.Category == topology.ServiceManagerCategory {
			continue
		}

		handler := newProtocolHandler(t, configured.Type)
		handler.SetConfig(handlerConfig.New(
			handlerConfig.HandlerType(configured.Type),
			configured.Endpoint.Id,
			configured.Category,
			configured.Endpoint.Port,
		))
		require.NoError(t, handler.Start())
		handlers = append(handlers, handler)
	}

	t.Cleanup(func() {
		for _, handler := range handlers {
			_ = closeProtocolHandler(handler)
		}
	})

	return handlers
}

func closeProtocolHandler(handler base.Interface) error {
	if err := closeHandler(handler); err != nil {
		return err
	}

	switch h := handler.(type) {
	case *sync_replier.SyncReplier:
		return closeHandler(h.Control)
	case *replier.Replier:
		return closeHandler(h.Control)
	case *publisher.Publisher:
		return closeHandler(h.Control)
	case *worker.Worker:
		return closeHandler(h.Control)
	default:
		return nil
	}
}

func newTestManager(t *testing.T, service topologyConfig.Service, managerEndpoint message.Endpoint) *Manager {
	t.Helper()

	manager, err := New(service.Name, managerEndpoint)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = manager.Close()
		time.Sleep(20 * time.Millisecond)
	})
	return manager
}

func requireHandlersStopped(t *testing.T, handlers []base.Interface) {
	t.Helper()

	require.Eventually(t, func() bool {
		for _, handler := range handlers {
			if handler.Socket() != nil || handler.Status() != base.SocketNil {
				return false
			}
		}
		return true
	}, time.Second, 10*time.Millisecond)
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

	for i, controlClient := range manager.handlerControls {
		handlerControlConfig, err := controlClient.HandlerConfig()
		require.NoError(t, err)

		expected := service.Handlers[i+1].AsHandler()
		require.Equal(t, expected.Category, handlerControlConfig.Category)
		require.Equal(t, expected.Endpoint.Id, handlerControlConfig.Id)
		require.Equal(t, expected.Endpoint.Port, handlerControlConfig.Port)
	}
}

func TestRemoteServicesReturnsConfiguredServices(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	serviceName := testEndpointID(t, "service")
	service := fakeServiceConfig(serviceName, managerEndpoint)
	startTestRuntimeHandler(t, service)

	manager := newTestManager(t, service, managerEndpoint)
	require.NoError(t, manager.Start())

	client, err := clientSyncReplier.NewClient(managerEndpoint.Id, managerEndpoint.Port)
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

	client, err := clientSyncReplier.NewClient(managerEndpoint.Id, managerEndpoint.Port)
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
	require.True(t, manager.Interface.Closed())
	require.Nil(t, manager.Interface.Socket())
	requireHandlersStopped(t, handlers)
}

func TestStartFailsWhenTopologyClientIsNil(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	manager, err := New("fake-service", managerEndpoint)
	require.NoError(t, err)
	manager.topologyClient = nil

	require.EqualError(t, manager.Start(), "setHandlerControls: topologyClient is nil")
	require.False(t, manager.Running())
}

func TestServiceNameValidation(t *testing.T) {
	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	manager, err := New("fake-service", managerEndpoint)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = manager.Close()
	})

	_, err = manager.StartService("other-service")
	require.EqualError(t, err, "service name is not empty and not equal to the service name")

	_, err = manager.IsServiceRunning("other-service")
	require.EqualError(t, err, "service name is not empty and not equal to the service name")

	require.EqualError(t, manager.StopService("other-service"), "service name is not empty and not equal to the service name")
}
