package service

import (
	"testing"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func TestSetHandlerConfigStoresByDefaultServiceAndCategory(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	handler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.ReplierType,
		Category: "main",
		Endpoint: message.NewEndpoint(testEndpointID(t, "main"), 0),
	}

	require.NoError(t, topologies.SetHandlerConfig(handler))

	serviceHandlers := requireHardcodedServiceHandlers(t, topologies, "custom-service")
	require.Equal(t, []topologyConfig.Handler{handler}, serviceHandlers)
}

func TestSetHandlerConfigStoresByExplicitServiceAndCategory(t *testing.T) {
	topologies := NewHardcodedTopologies("default-service")
	handler := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: "api",
		Endpoint: message.NewEndpoint(testEndpointID(t, "api"), 0),
	}

	require.NoError(t, topologies.SetHandlerConfig(handler, "other-service"))

	serviceHandlers := requireHardcodedServiceHandlers(t, topologies, "other-service")
	require.Equal(t, []topologyConfig.Handler{handler}, serviceHandlers)
	_, exists := topologies.handlerConfigs["default-service"]
	require.False(t, exists)
}

func TestSetHandlerConfigReplacesExistingCategory(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	first := topologyConfig.IndependentHandler{
		Type:     topologyConfig.ReplierType,
		Category: "main",
		Endpoint: message.NewEndpoint(testEndpointID(t, "first"), 0),
	}
	second := topologyConfig.IndependentHandler{
		Type:     topologyConfig.SyncReplierType,
		Category: "main",
		Endpoint: message.NewEndpoint(testEndpointID(t, "second"), 0),
	}

	require.NoError(t, topologies.SetHandlerConfig(first))
	require.NoError(t, topologies.SetHandlerConfig(second))

	serviceHandlers := requireHardcodedServiceHandlers(t, topologies, "custom-service")
	require.Equal(t, []topologyConfig.Handler{second}, serviceHandlers)
}

func TestSetServiceConfigStoresByServiceName(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	serviceConfig := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      "other-service",
		ModuleUrl: DefaultModuleUrl,
	}

	require.NoError(t, topologies.SetServiceConfig(serviceConfig, "other-service"))

	require.Equal(t, serviceConfig, topologies.serviceConfigs["other-service"])
}

func TestSetServiceParamsStoresByDefaultService(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")

	require.NoError(t, topologies.SetServiceParams(datatype.New().Set("api-key", "secret")))

	require.Equal(t, "secret", topologies.serviceParams["custom-service"]["api-key"])
}

func TestSetServiceParamsStoresByExplicitService(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")

	require.NoError(t, topologies.SetServiceParams(datatype.New().Set("model", "haiku"), "other-service"))

	require.Equal(t, "haiku", topologies.serviceParams["other-service"]["model"])
}

func TestSetServiceParamsMergesExistingKeys(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")

	require.NoError(t, topologies.SetServiceParams(datatype.New().Set("api-key", "first")))
	require.NoError(t, topologies.SetServiceParams(datatype.New().Set("model", "haiku")))

	params := topologies.serviceParams["custom-service"]
	require.Equal(t, "first", params["api-key"])
	require.Equal(t, "haiku", params["model"])
}

func TestSetCommandDepsStoresByDefaultHandlerAndService(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	dep := topologyConfig.DepService{Name: "account"}

	require.NoError(t, topologies.SetCommandDeps(dep))

	require.Equal(t, []topologyConfig.DepService{dep}, topologies.commandDeps["custom-service"]["main"])
}

func TestSetCommandDepsStoresByExplicitHandlerAndService(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	dep := topologyConfig.DepService{Name: "account"}

	require.NoError(t, topologies.SetCommandDeps(dep, "api", "other-service"))

	require.Equal(t, []topologyConfig.DepService{dep}, topologies.commandDeps["other-service"]["api"])
}

func TestSetCommandDepsReplacesExistingDepByName(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	first := topologyConfig.DepService{Name: "account"}
	second := topologyConfig.DepService{
		Name: "account",
		Proxies: []string{
			linkTarget("proxy"),
		},
	}

	require.NoError(t, topologies.SetCommandDeps(first))
	require.NoError(t, topologies.SetCommandDeps(second))

	require.Equal(t, []topologyConfig.DepService{second}, topologies.commandDeps["custom-service"]["main"])
}

func TestSetHandlerDepsStoresByDefaultService(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	dep := topologyConfig.DepService{Name: "account"}

	require.NoError(t, topologies.SetHandlerDeps(dep))

	require.Equal(t, []topologyConfig.DepService{dep}, topologies.handlerDeps["custom-service"])
}

func TestSetHandlerDepsStoresByExplicitService(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	dep := topologyConfig.DepService{Name: "account"}

	require.NoError(t, topologies.SetHandlerDeps(dep, "other-service"))

	require.Equal(t, []topologyConfig.DepService{dep}, topologies.handlerDeps["other-service"])
}

func TestSetHandlerDepsReplacesExistingDepByName(t *testing.T) {
	topologies := NewHardcodedTopologies("custom-service")
	first := topologyConfig.DepService{Name: "account"}
	second := topologyConfig.DepService{
		Name:       "account",
		Extensions: []string{linkTarget("extension")},
	}

	require.NoError(t, topologies.SetHandlerDeps(first))
	require.NoError(t, topologies.SetHandlerDeps(second))

	require.Equal(t, []topologyConfig.DepService{second}, topologies.handlerDeps["custom-service"])
}

func TestNewEmbedsHardcodedTopologies(t *testing.T) {
	independent, err := New("custom-service", testConfigPath(t))
	require.NoError(t, err)
	require.NotNil(t, independent.WithHardcodedTopology)
	require.Equal(t, "custom-service", independent.WithHardcodedTopology.mushroomURL)
}

func requireHardcodedServiceHandlers(t *testing.T, topologies *WithHardcodedTopology, serviceName string) []topologyConfig.Handler {
	t.Helper()

	serviceHandlers, exists := topologies.handlerConfigs[serviceName]
	require.True(t, exists)

	return serviceHandlers
}
