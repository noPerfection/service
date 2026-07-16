package handlers

import (
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/mushroom"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewSetup()

	require.NotNil(t, manager)
	require.NotNil(t, manager.handlers)
	require.Empty(t, manager.handlers)
}

func TestSetHandlerRegistersProtocolHandler(t *testing.T) {
	manager := NewSetup()
	handler := registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "sync")

	require.Same(t, handler, manager.handlers["sync"])
	require.Equal(t, protocolHandler.SyncReplierType, handler.Type())
}

func TestSetHandlerRejectsDuplicateCategory(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "api")

	second := newProtocolHandler(t, protocolHandler.ReplierType)
	setInprocHandlerEndpoint(t, second, testEndpointID(t, "api"))

	require.EqualError(t, manager.SetHandler("api", second), "handler of api category already exists")
}

func TestSetHandlerRejectsDuplicateAfterStart(t *testing.T) {
	manager := NewSetup()
	first := registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "api")
	require.NoError(t, manager.Start(testServiceMushroomURL))
	requireHandlerRunning(t, first)
	t.Cleanup(func() {
		require.NoError(t, manager.Close())
	})

	second := newProtocolHandler(t, protocolHandler.ReplierType)
	setInprocHandlerEndpoint(t, second, testEndpointID(t, "api-replace"))

	require.EqualError(t, manager.SetHandler("api", second), "handler of api category already exists")
	requireHandlerRunning(t, first)
}

func TestManagerRegistryCapacity(t *testing.T) {
	manager := NewSetup()

	cases := []struct {
		handlerType protocolHandler.HandlerType
		category    string
	}{
		{protocolHandler.SyncReplierType, "sync"},
		{protocolHandler.ReplierType, "async"},
		{protocolHandler.PublisherType, "pub"},
		{protocolHandler.PairType, "pair"},
		{protocolHandler.WorkerType, "worker"},
	}

	for _, tc := range cases {
		registerInprocHandler(t, manager, tc.handlerType, tc.category)
	}

	require.Len(t, manager.handlers, len(cases))
	for _, tc := range cases {
		handler := manager.handlers[tc.category]
		require.Equal(t, tc.handlerType, handler.Type())
	}
}

func TestSetLogger(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "sync")

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.NoError(t, manager.SetLogger(logger))
	require.Same(t, logger, manager.logger)
}

func TestSetLoggerNilDisablesLogger(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "sync")

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.NoError(t, manager.SetLogger(logger))
	require.NoError(t, manager.SetLogger(nil))
	require.Nil(t, manager.logger)
}

func TestSetLoggerRejectsNilRegistryEntry(t *testing.T) {
	manager := NewSetup()
	manager.handlers["bad"] = nil

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.EqualError(t, manager.SetLogger(logger), "handler of bad category is nil")
}

func TestRouteUsesDefaultHandlerCategory(t *testing.T) {
	manager := NewSetup()

	require.NoError(t, manager.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New())
	}))

	require.Contains(t, manager.routes, DefaultHandlerCategory)
	require.Contains(t, manager.routes[DefaultHandlerCategory], "hello")
}

func TestStartRejectsRouteForMissingCategory(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, protocolHandler.SyncReplierType, DefaultHandlerCategory)
	require.NoError(t, manager.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New())
	}, "missing"))

	require.EqualError(t, manager.Start(testServiceMushroomURL), "routed to a category that not exist: 'missing'")
}

func TestRouteRejectsAfterStart(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, protocolHandler.SyncReplierType, DefaultHandlerCategory)
	require.NoError(t, manager.Start(testServiceMushroomURL))
	t.Cleanup(func() {
		require.NoError(t, manager.Close())
	})

	err := manager.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New())
	})
	require.EqualError(t, err, "I cant route when its already started. Please stop the handler first or the best way to route before starting the handler")
}

func TestRouteIsUsedByStartedHandler(t *testing.T) {
	manager := NewSetup()
	handler := registerInprocHandler(t, manager, protocolHandler.SyncReplierType, DefaultHandlerCategory)
	require.NoError(t, manager.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		name, err := req.RouteParameters().StringValue("name")
		if err != nil {
			return req.Fail(err.Error())
		}
		return req.Ok(datatype.New().Set("reply", "hello "+name))
	}))

	require.NoError(t, manager.Start(testServiceMushroomURL))
	t.Cleanup(func() {
		require.NoError(t, manager.Close())
	})

	endpoint := handler.Endpoint()
	client, err := protocolClient.New(endpoint.Id, endpoint.Port, protocolClient.SyncReplierType)
	require.NoError(t, err)
	client.Timeout(time.Second)
	client.Attempt(1)
	defer client.Close()

	reply, err := client.Request(&message.Request{
		Command:    "hello",
		Parameters: datatype.New().Set("name", "route"),
	})
	require.NoError(t, err)
	require.True(t, reply.IsOK(), reply.ErrorMessage())
	replyText, err := reply.ReplyParameters().StringValue("reply")
	require.NoError(t, err)
	require.Equal(t, "hello route", replyText)
}

func TestStartNoHandlers(t *testing.T) {
	manager := NewSetup()

	require.EqualError(t, manager.Start(testServiceMushroomURL), "no handlers")
}

func TestStartRequiresHandlerConfig(t *testing.T) {
	manager := NewSetup()
	handler := protocolHandler.NewSyncReplier()
	require.NoError(t, manager.SetHandler("sync", handler))

	require.EqualError(t, manager.Start(testServiceMushroomURL), "handler of sync category has no config")
}

func TestStartReturnsHandlerStartError(t *testing.T) {
	manager := NewSetup()

	sharedEndpoint := testEndpointID(t, "sync-bind")
	blocker := protocolHandler.NewSyncReplier()
	setInprocHandlerEndpoint(t, blocker, sharedEndpoint)
	mushroomURL, err := mushroom.New(testServiceMushroomURL, "blocker")
	require.NoError(t, err)
	blocker.SetMushroomURL(mushroomURL.String())
	require.NoError(t, blocker.Start())
	t.Cleanup(func() {
		handlers := []protocolHandler.Interface{blocker}
		for _, handler := range handlers {
			require.NoError(t, CloseViaControl(handler))
		}
	})

	handler := protocolHandler.NewSyncReplier()
	setInprocHandlerEndpoint(t, handler, sharedEndpoint)
	require.NoError(t, manager.SetHandler("sync", handler))

	handlerLink, err := mushroom.New(testServiceMushroomURL, "sync")
	require.NoError(t, err)
	handler.SetMushroomURL(handlerLink.String())
	err = handler.Start()
	require.Error(t, err)
	require.Contains(t, err.Error(), "control.Start")
}

func TestStartWithMultipleProtocolHandlers(t *testing.T) {
	manager := NewSetup()

	registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "sync")
	registerInprocHandler(t, manager, protocolHandler.ReplierType, "async")
	registerInprocHandler(t, manager, protocolHandler.PublisherType, "pub")
	registerInprocHandler(t, manager, protocolHandler.WorkerType, "worker")

	require.NoError(t, manager.Start(testServiceMushroomURL))
	t.Cleanup(func() {
		require.NoError(t, manager.Close())
	})

	for category, handler := range manager.handlers {
		requireHandlerRunning(t, handler)
		_ = category
	}
}

func TestCloseNoHandlers(t *testing.T) {
	manager := NewSetup()

	require.NoError(t, manager.Close())
}

func TestCloseRejectsNilRegistryEntry(t *testing.T) {
	manager := NewSetup()
	manager.handlers["bad"] = nil

	require.EqualError(t, manager.Close(), "handler of bad category is nil")
}

func TestCloseMarksHandlersClosed(t *testing.T) {
	manager := NewSetup()
	handler := registerInprocHandler(t, manager, protocolHandler.SyncReplierType, "sync")

	require.NoError(t, manager.Start(testServiceMushroomURL))
	require.NoError(t, manager.Close())
	requireHandlerClosed(t, handler)
}
