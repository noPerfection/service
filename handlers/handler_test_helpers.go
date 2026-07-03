package handlers

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/handler/pair"
	"github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/handler/worker"
	"github.com/noPerfection/protocol/message"
	"github.com/stretchr/testify/require"
)

var testEndpointSeq atomic.Uint64

func newProtocolHandler(t *testing.T, handlerType base.HandlerType) base.Interface {
	t.Helper()

	switch handlerType {
	case base.SyncReplierType:
		return sync_replier.New()
	case base.ReplierType:
		return replier.New()
	case base.PublisherType:
		return publisher.New()
	case base.PairType:
		return pair.New()
	case base.WorkerType:
		return worker.New()
	default:
		t.Fatalf("unsupported handler type: %v", handlerType)
		return nil
	}
}

func testEndpointID(t *testing.T, category string) string {
	t.Helper()
	seq := testEndpointSeq.Add(1)
	return fmt.Sprintf("%s_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), category, seq)
}

func setInprocHandlerEndpoint(t *testing.T, handler base.Interface, endpointID string) message.Endpoint {
	t.Helper()

	endpoint := message.NewEndpoint(endpointID, 0)
	handler.SetEndpoint(endpoint)
	return endpoint
}

func registerInprocHandler(t *testing.T, manager *Handlers, handlerType base.HandlerType, category string) base.Interface {
	t.Helper()

	handler := newProtocolHandler(t, handlerType)
	setInprocHandlerEndpoint(t, handler, testEndpointID(t, category))
	require.NoError(t, manager.SetHandler(category, handler))
	return handler
}

func handlerControlEndpoint(handler base.Interface) message.Endpoint {
	return control.NewInternalControlEndpoint(handler.Endpoint())
}

func newTestHandlerControl(t *testing.T, handler base.Interface) *clientSyncReplier.BaseControl {
	t.Helper()

	endpoint := handlerControlEndpoint(handler)
	controlClient, err := clientSyncReplier.NewBaseControl(endpoint.Id, endpoint.Port)
	require.NoError(t, err)
	controlClient.Timeout(time.Second)
	controlClient.Attempt(3)
	return controlClient
}

func requireHandlerStatus(t *testing.T, handler base.Interface, expected string) {
	t.Helper()

	controlClient := newTestHandlerControl(t, handler)
	defer controlClient.Close()

	require.Eventually(t, func() bool {
		status, err := controlClient.HandlerStatus()
		return err == nil && status == expected
	}, 2*time.Second, 10*time.Millisecond)
}

func requireHandlerRunning(t *testing.T, handler base.Interface) {
	t.Helper()
	requireHandlerStatus(t, handler, base.SocketReady)
}

func requireHandlerClosed(t *testing.T, handler base.Interface) {
	t.Helper()
	requireHandlerStatus(t, handler, base.SocketNil)
}
