package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

var testEndpointSeq atomic.Uint64

func testEndpointID(t *testing.T, name string) string {
	t.Helper()
	seq := testEndpointSeq.Add(1)
	return fmt.Sprintf("%s_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), name, seq)
}

func testConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "noPerfection.json")
}

func closeTopologyHandler(t *testing.T) {
	t.Helper()

	controlConfig := control.CreateInternalConfig(topology.HandlerConfig())
	controlClient, err := clientSyncReplier.NewClient(controlConfig.Id, controlConfig.Port)
	if err == nil {
		_, _ = controlClient.Request(&message.Request{
			Command:    control.HandlerClose,
			Parameters: datatype.New(),
		})
		_ = controlClient.Close()
	}
	time.Sleep(100 * time.Millisecond)
}

func requireServiceHandler(t *testing.T, service topologyConfig.Service, category string) topologyConfig.Handler {
	t.Helper()

	handler, err := service.HandlerByCategory(category)
	require.NoError(t, err)
	return handler
}

func TestNewDefaultParamsLintDefaultTopologyCreatesDefaultService(t *testing.T) {
	independent, err := New(nil, testConfigPath(t))
	require.NoError(t, err)
	require.Equal(t, DefaultName, independent.Name())

	require.NoError(t, independent.lintDefaultTopology())

	serviceConfig, err := independent.topologyHandler.Service(DefaultName)
	require.NoError(t, err)
	require.Equal(t, topologyConfig.IndependentType, serviceConfig.Type)
	require.Len(t, serviceConfig.Handlers, 1)

	defaultHandler := serviceConfig.Handlers[0]
	require.Equal(t, topologyConfig.ReplierType, defaultHandler.Type)
	require.Equal(t, handlers.DefaultHandlerCategory, defaultHandler.Category)
	require.Equal(t, handlers.DefaultHandlerEndpoint, defaultHandler.Endpoint)

	require.True(t, independent.Handlers.IsHandlerExist(handlers.DefaultHandlerCategory))
}

func TestLintManagerTopologyOverwritesExistingManagerConfig(t *testing.T) {
	configPath := testConfigPath(t)
	existingManager := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: topology.ServiceManagerCategory,
		Endpoint: DefaultServiceManagerEndpoint,
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers: []topologyConfig.Handler{
			{
				Type:     topologyConfig.ReplierType,
				Category: handlers.DefaultHandlerCategory,
				Endpoint: handlers.DefaultHandlerEndpoint,
			},
			existingManager,
		},
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	managerEndpoint := message.NewEndpoint(testEndpointID(t, "manager"), 0)
	independent, err := New("custom-service", configPath, managerEndpoint)
	require.NoError(t, err)

	require.NoError(t, independent.lintManagerTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	managerHandler := requireServiceHandler(t, serviceConfig, topology.ServiceManagerCategory)
	require.Equal(t, topologyConfig.SyncReplierType, managerHandler.Type)
	require.Equal(t, managerEndpoint, managerHandler.Endpoint)
}

func TestLintDefaultTopologyKeepsExistingDefaultHandlerConfig(t *testing.T) {
	configPath := testConfigPath(t)
	existingMain := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: handlers.DefaultHandlerCategory,
		Endpoint: message.NewEndpoint(testEndpointID(t, "existing-main"), 0),
	}
	existingService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "custom-service",
		ModuleUrl: DefaultModuleUrl,
		Handlers:  []topologyConfig.Handler{existingMain},
	}
	appConfig, err := topologyConfig.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.SetService(existingService))
	require.NoError(t, appConfig.Save())

	independent, err := New("custom-service", configPath, message.NewEndpoint(testEndpointID(t, "manager"), 0))
	require.NoError(t, err)

	require.NoError(t, independent.lintDefaultTopology())

	serviceConfig, err := independent.topologyHandler.Service("custom-service")
	require.NoError(t, err)
	mainHandler := requireServiceHandler(t, serviceConfig, handlers.DefaultHandlerCategory)
	require.Equal(t, existingMain.Endpoint, mainHandler.Endpoint)
}

func TestStartCreatesDefaultHandlerAndStartsManager(t *testing.T) {
	independent, err := New(
		"custom-service",
		testConfigPath(t),
		message.NewEndpoint(testEndpointID(t, "manager"), 0),
	)
	require.NoError(t, err)

	require.NoError(t, independent.Start())
	t.Cleanup(func() {
		_ = independent.Stop()
		closeTopologyHandler(t)
	})

	require.True(t, independent.manager.Running())

	topologyClient, err := topology.NewClient()
	require.NoError(t, err)
	defer topologyClient.Close()

	serviceConfig, err := topologyClient.Service("custom-service")
	require.NoError(t, err)
	mainHandler := requireServiceHandler(t, serviceConfig, handlers.DefaultHandlerCategory)
	require.Equal(t, handlers.DefaultHandlerEndpoint, mainHandler.Endpoint)
}

func TestNewRejectsInvalidParams(t *testing.T) {
	_, err := New("service", testConfigPath(t), message.NewEndpoint("manager", 0), "extra")
	require.EqualError(t, err, "too many arguments, expected name, config path, and manager endpoint")

	_, err = New(10)
	require.EqualError(t, err, "name argument must be string")

	_, err = New("service", 10)
	require.EqualError(t, err, "config path argument must be string")

	_, err = New("service", testConfigPath(t), "manager")
	require.EqualError(t, err, "manager endpoint argument must be message.Endpoint")
}
