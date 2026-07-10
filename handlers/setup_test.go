package handlers

import (
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/message"
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
	handler := registerInprocHandler(t, manager, base.SyncReplierType, "sync")

	require.Same(t, handler, manager.handlers["sync"])
	require.Equal(t, base.SyncReplierType, handler.Type())
}

func TestSetHandlerRejectsDuplicateCategory(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, base.SyncReplierType, "api")

	second := newProtocolHandler(t, base.ReplierType)
	setInprocHandlerEndpoint(t, second, testEndpointID(t, "api"))

	require.EqualError(t, manager.SetHandler("api", second), "handler of api category already exists")
}

func TestSetHandlerRejectsDuplicateAfterStart(t *testing.T) {
	manager := NewSetup()
	first := registerInprocHandler(t, manager, base.SyncReplierType, "api")
	require.NoError(t, manager.Start())
	requireHandlerRunning(t, first)
	t.Cleanup(func() {
		require.NoError(t, manager.Close())
	})

	second := newProtocolHandler(t, base.ReplierType)
	setInprocHandlerEndpoint(t, second, testEndpointID(t, "api-replace"))

	require.EqualError(t, manager.SetHandler("api", second), "handler of api category already exists")
	requireHandlerRunning(t, first)
}

func TestManagerRegistryCapacity(t *testing.T) {
	manager := NewSetup()

	cases := []struct {
		handlerType base.HandlerType
		category    string
	}{
		{base.SyncReplierType, "sync"},
		{base.ReplierType, "async"},
		{base.PublisherType, "pub"},
		{base.PairType, "pair"},
		{base.WorkerType, "worker"},
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
	registerInprocHandler(t, manager, base.SyncReplierType, "sync")

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.NoError(t, manager.SetLogger(logger))
	require.Same(t, logger, manager.logger)
}

func TestSetLoggerNilDisablesLogger(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, base.SyncReplierType, "sync")

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
	registerInprocHandler(t, manager, base.SyncReplierType, DefaultHandlerCategory)
	require.NoError(t, manager.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New())
	}, "missing"))

	require.EqualError(t, manager.Start(), "routed to a category that not exist: 'missing'")
}

func TestRouteRejectsAfterStart(t *testing.T) {
	manager := NewSetup()
	registerInprocHandler(t, manager, base.SyncReplierType, DefaultHandlerCategory)
	require.NoError(t, manager.Start())
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
	handler := registerInprocHandler(t, manager, base.SyncReplierType, DefaultHandlerCategory)
	require.NoError(t, manager.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		name, err := req.RouteParameters().StringValue("name")
		if err != nil {
			return req.Fail(err.Error())
		}
		return req.Ok(datatype.New().Set("reply", "hello "+name))
	}))

	require.NoError(t, manager.Start())
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

	require.EqualError(t, manager.Start(), "no handlers")
}

func TestStartRequiresHandlerConfig(t *testing.T) {
	manager := NewSetup()
	handler := sync_replier.New()
	require.NoError(t, manager.SetHandler("sync", handler))

	require.EqualError(t, manager.Start(), "handler of sync category has no config")
}

func TestStartReturnsHandlerStartError(t *testing.T) {
	manager := NewSetup()

	sharedEndpoint := testEndpointID(t, "sync-bind")
	blocker := sync_replier.New()
	setInprocHandlerEndpoint(t, blocker, sharedEndpoint)
	require.NoError(t, blocker.Start())
	t.Cleanup(func() {
		handlers := []base.Interface{blocker}
		for _, handler := range handlers {
			require.NoError(t, CloseViaControl(handler))
		}
	})

	handler := sync_replier.New()
	setInprocHandlerEndpoint(t, handler, sharedEndpoint)
	require.NoError(t, manager.SetHandler("sync", handler))

	err := manager.Start()
	require.Error(t, err)
	require.ErrorContains(t, err, "handler(category: 'sync').Start:")
}

func TestStartWithMultipleProtocolHandlers(t *testing.T) {
	manager := NewSetup()

	registerInprocHandler(t, manager, base.SyncReplierType, "sync")
	registerInprocHandler(t, manager, base.ReplierType, "async")
	registerInprocHandler(t, manager, base.PublisherType, "pub")
	registerInprocHandler(t, manager, base.WorkerType, "worker")

	require.NoError(t, manager.Start())
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
	handler := registerInprocHandler(t, manager, base.SyncReplierType, "sync")

	require.NoError(t, manager.Start())
	require.NoError(t, manager.Close())
	requireHandlerClosed(t, handler)
}
