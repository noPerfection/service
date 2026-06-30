package service

import (
	"testing"

	"github.com/noPerfection/protocol/message"
	"github.com/stretchr/testify/require"
)

func writeInprocExtensionTopology(t *testing.T, services ...Config) string {
	t.Helper()
	path := testConfigPath(t)
	handler, err := newTopologyHandler(path)
	require.NoError(t, err)
	for _, service := range services {
		require.NoError(t, handler.AddService(service))
	}
	return path
}

func inprocProxyService(name string) Config {
	return Config{
		Type:      ProxyType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		Handlers: []Handler{
			ProxyHandler{
				IndependentHandler: IndependentHandler{
					Type:     SyncReplierType,
					Category: DefaultHandlerCategory,
					Endpoint: message.NewEndpoint(name, 0),
				},
			},
		},
	}
}

func ipcProxyService(name string) Config {
	return Config{
		Type:         ProxyType,
		Name:         name,
		ModuleUrl:    DefaultModuleUrl,
		StartCommand: "/bin/true",
		Handlers: []Handler{
			ProxyHandler{
				IndependentHandler: IndependentHandler{
					Type:     SyncReplierType,
					Category: DefaultHandlerCategory,
					Endpoint: message.NewEndpoint("tmp/"+name, 0),
				},
			},
		},
	}
}

func TestStartServiceRejectsNonInproc(t *testing.T) {
	path := writeInprocExtensionTopology(t, ipcProxyService("ipc-proxy"))
	ext, err := NewInprocExtension(path)
	require.NoError(t, err)

	proxy, err := NewProxy("ipc-proxy", path)
	require.NoError(t, err)

	require.NoError(t, ext.SetService("ipc-proxy", proxy))

	_, err = ext.startService("ipc-proxy")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not inproc")
}

func TestSetServiceAcceptsRegisteredTypes(t *testing.T) {
	path := writeInprocExtensionTopology(t,
		defaultInprocTopologyExtensionServiceConfig(),
		inprocProxyService("inproc-proxy"),
		Config{
			Type:      IndependentType,
			Name:      "host",
			ModuleUrl: DefaultModuleUrl,
			Handlers: []Handler{
				IndependentHandler{
					Type:     SyncReplierType,
					Category: DefaultHandlerCategory,
					Endpoint: message.NewEndpoint("host", 0),
				},
			},
		},
	)
	ext, err := NewInprocExtension(path)
	require.NoError(t, err)

	proxy, err := NewProxy("inproc-proxy", path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService("inproc-proxy", proxy))

	independent, err := New("host", path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService("host", independent))

	extension, err := NewExt(InprocTopologyServiceName, path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService(InprocTopologyServiceName, extension))

	err = ext.SetService("inproc-proxy", nil)
	require.Error(t, err)
}

func TestInprocTopologyRegistryLifecycle(t *testing.T) {
	path := writeInprocExtensionTopology(t, inprocProxyService("child"))
	ext, err := NewInprocExtension(path)
	require.NoError(t, err)

	proxy, err := NewProxy("child", path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService("child", proxy))

	id, err := ext.startService("child")
	require.NoError(t, err)
	require.NotEmpty(t, id)

	handler, err := newTopologyHandler(path)
	require.NoError(t, err)
	serviceConfig, err := handler.Service("child")
	require.NoError(t, err)

	running, err := ProbeInprocServiceRunning(serviceConfig)
	require.NoError(t, err)
	require.True(t, running)

	require.NoError(t, proxy.Stop())

	running, err = ProbeInprocServiceRunning(serviceConfig)
	require.NoError(t, err)
	require.False(t, running)
}
