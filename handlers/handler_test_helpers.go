package handlers

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/stretchr/testify/require"
)

var testEndpointSeq atomic.Uint64

func newProtocolHandler(t *testing.T, handlerType protocolHandler.HandlerType) protocolHandler.Interface {
	t.Helper()

	switch handlerType {
	case protocolHandler.SyncReplierType:
		return protocolHandler.NewSyncReplier()
	case protocolHandler.ReplierType:
		return protocolHandler.NewReplier()
	case protocolHandler.PublisherType:
		return protocolHandler.NewPublisher()
	case protocolHandler.PairType:
		return protocolHandler.NewPair()
	case protocolHandler.WorkerType:
		return protocolHandler.NewWorker()
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

func setInprocHandlerEndpoint(t *testing.T, handler protocolHandler.Interface, endpointID string) message.Endpoint {
	t.Helper()

	endpoint := message.NewEndpoint(endpointID, 0)
	handler.SetEndpoint(endpoint)
	return endpoint
}

const testServiceMushroomURL = "*pkg:$?var=services[name:test-service]"

func registerInprocHandler(t *testing.T, manager *Setup, handlerType protocolHandler.HandlerType, category string) protocolHandler.Interface {
	t.Helper()

	handler := newProtocolHandler(t, handlerType)
	setInprocHandlerEndpoint(t, handler, testEndpointID(t, category))
	require.NoError(t, manager.SetHandler(category, handler))
	return handler
}

func handlerControlEndpoint(handler protocolHandler.Interface) message.Endpoint {
	return protocolHandler.NewInternalControlEndpoint(handler.Endpoint())
}

func newTestHandlerControl(t *testing.T, handler protocolHandler.Interface) *protocolClient.Control {
	t.Helper()

	endpoint := handlerControlEndpoint(handler)
	controlClient, err := protocolClient.NewControl(endpoint.Id, endpoint.Port)
	require.NoError(t, err)
	controlClient.Timeout(time.Second)
	controlClient.Attempt(3)
	return controlClient
}

func requireHandlerStatus(t *testing.T, handler protocolHandler.Interface, expected string) {
	t.Helper()

	controlClient := newTestHandlerControl(t, handler)
	defer controlClient.Close()

	require.Eventually(t, func() bool {
		status, err := controlClient.HandlerStatus()
		return err == nil && status == expected
	}, 2*time.Second, 10*time.Millisecond)
}

func requireHandlerRunning(t *testing.T, handler protocolHandler.Interface) {
	t.Helper()
	requireHandlerStatus(t, handler, protocolHandler.SocketReady)
}

func requireHandlerClosed(t *testing.T, handler protocolHandler.Interface) {
	t.Helper()
	requireHandlerStatus(t, handler, protocolHandler.SocketNil)
}
