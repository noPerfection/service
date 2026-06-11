package service

import (
	"testing"

	"github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func TestProxyAddDefaultServiceToTopologyFillsModuleURL(t *testing.T) {
	stubBuildInfo(t, "example.com/app", true)
	proxy, err := NewProxy("default-name-proxy", testConfigPath(t))
	require.NoError(t, err)

	require.NoError(t, proxy.addDefaultServiceToTopology())

	serviceConfig, err := proxy.topologyHandler.Service("default-name-proxy")
	require.NoError(t, err)
	require.Equal(t, config.ProxyType, serviceConfig.Type)
	require.Equal(t, "example.com/app", serviceConfig.ModuleUrl)
}
