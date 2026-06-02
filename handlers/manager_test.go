package handlers

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/pair"
	"github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/handler/worker"
	"github.com/stretchr/testify/require"
)

var testEndpointSeq atomic.Uint64

func newProtocolHandler(t *testing.T, handlerType config.HandlerType) base.Interface {
	t.Helper()

	switch handlerType {
	case config.SyncReplierType:
		return sync_replier.New()
	case config.ReplierType:
		return replier.New()
	case config.PublisherType:
		return publisher.New()
	case config.PairType:
		return pair.New()
	case config.WorkerType:
		return worker.New()
	default:
		t.Fatalf("unsupported handler type: %v", handlerType)
		return nil
	}
}

func inprocHandlerConfig(handlerType config.HandlerType, category string, endpointID string) *config.Handler {
	return config.New(handlerType, endpointID, category, 0)
}

func testEndpointID(t *testing.T, category string) string {
	t.Helper()
	seq := testEndpointSeq.Add(1)
	return fmt.Sprintf("%s_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), category, seq)
}

func registerInprocHandler(t *testing.T, manager *Manager, handlerType config.HandlerType, category string) base.Interface {
	t.Helper()

	handler := newProtocolHandler(t, handlerType)
	handler.SetConfig(inprocHandlerConfig(handlerType, category, testEndpointID(t, category)))
	require.NoError(t, manager.SetHandler(category, handler))
	return handler
}

func TestNewManager(t *testing.T) {
	manager := NewManager()

	require.NotNil(t, manager)
	require.NotNil(t, manager.handlers)
	require.Empty(t, manager.handlers)
}

func TestSetHandlerRegistersProtocolHandler(t *testing.T) {
	manager := NewManager()
	handler := registerInprocHandler(t, manager, config.SyncReplierType, "sync")

	require.Same(t, handler, manager.handlers["sync"])
	require.Equal(t, config.SyncReplierType, handler.Type())
}

func TestSetHandlerDuplicateCategoryOverwritesWithoutError(t *testing.T) {
	manager := NewManager()
	first := registerInprocHandler(t, manager, config.SyncReplierType, "api")
	second := registerInprocHandler(t, manager, config.ReplierType, "api")

	require.Len(t, manager.handlers, 1)
	require.Same(t, second, manager.handlers["api"])
	require.NotSame(t, first, manager.handlers["api"])
	require.Equal(t, config.ReplierType, manager.handlers["api"].(base.Interface).Type())
	require.True(t, first.Closed())
}

func TestSetHandlerStopsRunningDuplicateCategoryBeforeReplacing(t *testing.T) {
	manager := NewManager()
	first := registerInprocHandler(t, manager, config.SyncReplierType, "api")
	require.NoError(t, manager.Start())
	require.False(t, first.Closed())

	second := newProtocolHandler(t, config.ReplierType)
	second.SetConfig(inprocHandlerConfig(config.ReplierType, "api", testEndpointID(t, "api")))

	require.NoError(t, manager.SetHandler("api", second))

	require.True(t, first.Closed())
	require.Nil(t, first.Socket())
	require.Same(t, second, manager.handlers["api"])
	require.False(t, second.Closed())
}

func TestManagerRegistryCapacity(t *testing.T) {
	manager := NewManager()

	cases := []struct {
		handlerType config.HandlerType
		category    string
	}{
		{config.SyncReplierType, "sync"},
		{config.ReplierType, "async"},
		{config.PublisherType, "pub"},
		{config.PairType, "pair"},
		{config.WorkerType, "worker"},
	}

	for _, tc := range cases {
		registerInprocHandler(t, manager, tc.handlerType, tc.category)
	}

	require.Len(t, manager.handlers, len(cases))
	for _, tc := range cases {
		handler, ok := manager.handlers[tc.category].(base.Interface)
		require.True(t, ok)
		require.Equal(t, tc.handlerType, handler.Type())
	}
}

func TestSetLogger(t *testing.T) {
	manager := NewManager()
	registerInprocHandler(t, manager, config.SyncReplierType, "sync")

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.NoError(t, manager.SetLogger(logger))
	require.Same(t, logger, manager.logger)
}

func TestSetLoggerNilDisablesLogger(t *testing.T) {
	manager := NewManager()
	registerInprocHandler(t, manager, config.SyncReplierType, "sync")

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.NoError(t, manager.SetLogger(logger))
	require.NoError(t, manager.SetLogger(nil))
	require.Nil(t, manager.logger)
}

func TestSetLoggerRejectsInvalidRegistryEntry(t *testing.T) {
	manager := NewManager()
	manager.handlers.Set("bad", "not a handler")

	logger, err := log.New("test", true)
	require.NoError(t, err)

	require.EqualError(t, manager.SetLogger(logger), "handler of bad category is not a base.Interface")
}

func TestStartNoHandlers(t *testing.T) {
	manager := NewManager()

	require.EqualError(t, manager.Start(), "no handlers")
}

func TestStartRequiresHandlerConfig(t *testing.T) {
	manager := NewManager()
	handler := sync_replier.New()
	require.NoError(t, manager.SetHandler("sync", handler))

	require.EqualError(t, manager.Start(), "handler of sync category has no config")
}

func TestStartReturnsHandlerStartError(t *testing.T) {
	manager := NewManager()
	handler := sync_replier.New()
	handler.SetConfig(config.New(config.ReplierType, testEndpointID(t, "sync"), "sync", 0))
	require.NoError(t, manager.SetHandler("sync", handler))

	err := manager.Start()
	require.Error(t, err)
	require.ErrorContains(t, err, "handler(category: 'sync').Start:")
	require.ErrorContains(t, err, "SyncReplier")
}

func TestStartWithMultipleProtocolHandlers(t *testing.T) {
	manager := NewManager()

	registerInprocHandler(t, manager, config.SyncReplierType, "sync")
	registerInprocHandler(t, manager, config.ReplierType, "async")
	registerInprocHandler(t, manager, config.PublisherType, "pub")
	registerInprocHandler(t, manager, config.WorkerType, "worker")

	require.NoError(t, manager.Start())
	t.Cleanup(func() {
		require.NoError(t, manager.Close())
	})

	for category, raw := range manager.handlers {
		handler := raw.(base.Interface)
		require.False(t, handler.Closed(), "handler %s should be running", category)
	}
}

func TestCloseNoHandlers(t *testing.T) {
	manager := NewManager()

	require.NoError(t, manager.Close())
}

func TestCloseRejectsInvalidRegistryEntry(t *testing.T) {
	manager := NewManager()
	manager.handlers.Set("bad", "not a handler")

	require.EqualError(t, manager.Close(), "handler of bad category is not a base.Interface")
}

func TestCloseMarksHandlersClosed(t *testing.T) {
	manager := NewManager()
	handler := registerInprocHandler(t, manager, config.SyncReplierType, "sync")

	require.NoError(t, manager.Start())
	require.NoError(t, manager.Close())
	require.True(t, handler.Closed())
}

func TestCloseHandlersClosesStartedHandlers(t *testing.T) {
	manager := NewManager()
	first := registerInprocHandler(t, manager, config.SyncReplierType, "first")
	second := registerInprocHandler(t, manager, config.ReplierType, "second")

	require.NoError(t, manager.Start())
	require.NoError(t, closeHandlers([]base.Interface{first, second}))

	require.True(t, first.Closed())
	require.True(t, second.Closed())
}
