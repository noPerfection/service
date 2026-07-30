package service

import (
	"testing"

	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func requireAddDefaultProxyServiceToTopology(t *testing.T, proxy *Proxy, configPath string) {
	t.Helper()
	setProxyMushroomURL(t, proxy, configPath)
	tp := proxy.topology()
	if _, err := tp.Service(proxy.dereference()); err == nil {
		return
	}
	require.NoError(t, tp.AddService(config.Service{
		Type:      config.ProxyType,
		Name:      proxy.name,
		ModuleUrl: DefaultModuleUrl,
		Handlers:  []config.Handler{},
	}))
}

func TestProxyAddDefaultServiceToTopologyFillsModuleURL(t *testing.T) {
	configPath := testConfigPath(t)
	proxy, err := NewProxy("default-name-proxy")
	require.NoError(t, err)
	requireTopologyFilepath(t, proxy, configPath)

	requireAddDefaultProxyServiceToTopology(t, proxy, configPath)

	serviceConfig, err := proxy.topologyHandler.Service("default-name-proxy")
	require.NoError(t, err)
	require.Equal(t, config.ProxyType, serviceConfig.Type)
	require.Equal(t, DefaultModuleUrl, serviceConfig.ModuleUrl)
}

func TestAddHardcodedEndpointsToTopologyCreatesDefaultProxyManagerHandler(t *testing.T) {
	configPath := testConfigPath(t)
	serviceName := "default-name-proxy"
	endpoint := message.NewEndpoint(testEndpointID(t, "manager"), 4100)
	existingService := config.Service{
		Type:      config.ProxyType,
		Name:      serviceName,
		ModuleUrl: DefaultModuleUrl,
		Handlers:  []config.Handler{},
	}
	appConfig, err := config.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
	require.NoError(t, appConfig.Save())

	proxy, err := NewProxy(serviceName)
	require.NoError(t, err)
	requireTopologyFilepath(t, proxy, configPath)
	require.NoError(t, proxy.SetEndpoint(endpoint, config.ServiceManagerCategory))
	require.NoError(t, proxy.addHardcodedEndpointsToTopology(proxy.topology()))

	serviceConfig, err := proxy.topologyHandler.Service(serviceName)
	require.NoError(t, err)
	managerHandler := requireServiceHandler(t, serviceConfig, config.ServiceManagerCategory)
	require.Equal(t, endpoint, managerHandler.Endpoint)
	require.Equal(t, config.SyncReplierType, managerHandler.Type)
}

func TestAddHardcodedEndpointsToTopologyCreatesDefaultExtensionManagerHandler(t *testing.T) {
	configPath := testConfigPath(t)
	serviceName := "metrics-extension"
	endpoint := message.NewEndpoint(testEndpointID(t, "manager"), 4100)
	existingService := config.Service{
		Type:      config.ExtensionType,
		Name:      serviceName,
		ModuleUrl: DefaultModuleUrl,
		Handlers:  []config.Handler{},
	}
	appConfig, err := config.Load(configPath)
	require.NoError(t, err)
	require.NoError(t, appConfig.AddService(existingService, rootServicesParent))
	require.NoError(t, appConfig.Save())

	extension, err := NewExt(serviceName)
	require.NoError(t, err)
	requireTopologyFilepath(t, extension, configPath)
	require.NoError(t, extension.SetEndpoint(endpoint, config.ServiceManagerCategory))
	require.NoError(t, extension.addHardcodedEndpointsToTopology(extension.topology()))

	serviceConfig, err := extension.topologyHandler.Service(serviceName)
	require.NoError(t, err)
	managerHandler := requireServiceHandler(t, serviceConfig, config.ServiceManagerCategory)
	require.Equal(t, endpoint, managerHandler.Endpoint)
	require.Equal(t, config.SyncReplierType, managerHandler.Type)
}
