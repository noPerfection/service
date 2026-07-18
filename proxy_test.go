package service

import (
	"testing"

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
