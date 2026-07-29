package service

import (
	"testing"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
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

func inprocOutboundService(name string) Config {
	return Config{
		Type:      IndependentType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		Handlers: []Handler{
			IndependentHandler{
				Type:     SyncReplierType,
				Category: DefaultHandlerCategory,
				Endpoint: message.NewEndpoint(name, 0),
			},
		},
	}
}

func startableInprocProxyService(name, outboundName string) Config {
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
				Routes:    []string{message.Any},
				Outbounds: []string{outboundLink(outboundName, DefaultHandlerCategory)},
			},
		},
	}
}

func inprocProxyOKRoute(req handlers.ProxyRequest) handlers.ProxyReply {
	return handlers.ProxyReply{Reply: *req.Ok(datatype.New()).(*message.Reply)}
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
	ext, err := NewInprocExtension()
	require.NoError(t, err)
	requireTopologyFilepath(t, ext, path)

	proxy, err := NewProxy("ipc-proxy")
	require.NoError(t, err)
	requireTopologyFilepath(t, proxy, path)
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
	ext, err := NewInprocExtension()
	require.NoError(t, err)
	requireTopologyFilepath(t, ext, path)

	proxy, err := NewProxy("inproc-proxy")
	require.NoError(t, err)
	requireTopologyFilepath(t, proxy, path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService("inproc-proxy", proxy))

	independent, err := New("host")
	require.NoError(t, err)
	requireTopologyFilepath(t, independent, path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService("host", independent))

	extension, err := NewExt(InprocTopologyServiceName)
	require.NoError(t, err)
	requireTopologyFilepath(t, extension, path)
	require.NoError(t, err)
	require.NoError(t, ext.SetService(InprocTopologyServiceName, extension))

	err = ext.SetService("inproc-proxy", nil)
	require.Error(t, err)
}

func TestInprocTopologyRegistryLifecycle(t *testing.T) {
	requireIsolatedTopologyHandler(t)

	const (
		childProxyName    = "child"
		childOutboundName = "child-outbound"
	)
	path := writeInprocExtensionTopology(t,
		inprocOutboundService(childOutboundName),
		startableInprocProxyService(childProxyName, childOutboundName),
	)
	ext, err := NewInprocExtension()
	require.NoError(t, err)
	requireTopologyFilepath(t, ext, path)
	t.Cleanup(func() {
		closeTopologyHandler(t)
	})

	proxy, err := NewProxy(childProxyName)
	require.NoError(t, err)
	requireTopologyFilepath(t, proxy, path)
	require.NoError(t, err)
	require.NoError(t, proxy.Route(message.Any, inprocProxyOKRoute, DefaultHandlerCategory))
	require.NoError(t, ext.SetService(childProxyName, proxy))

	id, err := ext.startService(childProxyName)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	require.NoError(t, proxy.Stop())
}
