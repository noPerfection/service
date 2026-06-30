package service

import (
	"testing"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func TestValidateInprocServiceManagers(t *testing.T) {
	tests := []struct {
		name    string
		service topologyConfig.Service
		wantErr string
	}{
		{
			name: "proxy tcp handler marked inproc requires inproc manager",
			service: inprocManagerProxyService(
				"proxy",
				message.NewEndpoint("localhost", 8001),
				[]string{handlers.DefaultHandlerCategory},
				message.NewEndpoint("localhost", 9001),
			),
			wantErr: `service "proxy" is inproc but manager endpoint`,
		},
		{
			name: "proxy tcp handler without inproc parameter skips manager validation",
			service: inprocManagerProxyService(
				"proxy",
				message.NewEndpoint("localhost", 8001),
				nil,
				message.NewEndpoint("localhost", 9001),
			),
		},
		{
			name: "inproc main handler requires inproc manager",
			service: inprocManagerIndependentService(
				"service",
				message.NewEndpoint("main", 0),
				message.NewEndpoint("localhost", 9001),
			),
			wantErr: `service "service" is inproc but manager endpoint`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.service.ValidateInprocServiceManager()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInprocessDepNumberIncludesManagerHandlerDep(t *testing.T) {
	independent, err := New("custom-service")
	require.NoError(t, err)
	requireTopologyFilepath(t, independent, testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(inprocManagerIndependentService(
		"custom-service",
		message.NewEndpoint("main", 0),
		message.NewEndpoint("custom-service_manager", 0),
	)))
	require.NoError(t, independent.addHardcodedServicesToTopology())
	require.NoError(t, independent.topologyHandler.AddService(defaultAiExtensionServiceConfig()))
	require.NoError(t, independent.SetHandlerDeps(topologyConfig.DepService{
		Name:       topology.ServiceManagerCategory,
		Extensions: []string{aiExtensionServiceLink()},
	}))
	require.NoError(t, independent.addHardcodedHandlerDepsToTopology())

	inprocServices, err := independent.topologyHandler.InprocessDepNumber("custom-service")
	require.NoError(t, err)
	require.Equal(t, 1, inprocServices, "ai manager handler-dep is counted")
}

func TestInprocessDepNumberSkipsNonInprocManagerHandlerDep(t *testing.T) {
	independent, err := New("custom-service")
	require.NoError(t, err)
	requireTopologyFilepath(t, independent, testConfigPath(t))
	require.NoError(t, err)
	require.NoError(t, independent.SetServiceConfig(inprocManagerIndependentService(
		"custom-service",
		message.NewEndpoint("main", 0),
		message.NewEndpoint("custom-service_manager", 0),
	)))
	require.NoError(t, independent.addHardcodedServicesToTopology())

	remoteAI := defaultAiExtensionServiceConfig()
	remoteAI.Handlers = []topologyConfig.Handler{
		topologyConfig.IndependentHandler{
			Type:     topologyConfig.SyncReplierType,
			Category: topology.ServiceManagerCategory,
			Endpoint: message.NewEndpoint("localhost", 9001),
		},
		topologyConfig.ExtensionHandler{
			IndependentHandler: topologyConfig.IndependentHandler{
				Type:     topologyConfig.ReplierType,
				Category: "main",
				Endpoint: message.NewEndpoint("localhost", 8001),
			},
		},
	}
	require.NoError(t, independent.topologyHandler.AddService(remoteAI))
	require.False(t, remoteAI.IsInproc())

	require.NoError(t, independent.SetHandlerDeps(topologyConfig.DepService{
		Name:       topology.ServiceManagerCategory,
		Extensions: []string{aiExtensionServiceLink()},
	}))
	require.NoError(t, independent.addHardcodedHandlerDepsToTopology())

	inprocServices, err := independent.topologyHandler.InprocessDepNumber("custom-service")
	require.NoError(t, err)
	require.Equal(t, 0, inprocServices)
}

func inprocManagerProxyService(name string, handlerEndpoint message.Endpoint, inprocHandlers []string, managerEndpoint message.Endpoint) topologyConfig.Service {
	parameters := datatype.KeyValue(nil)
	if len(inprocHandlers) > 0 {
		parameters = datatype.New().Set(topologyConfig.InprocHandlersParameter, inprocHandlers)
	}
	return topologyConfig.Service{
		Type:       topologyConfig.ProxyType,
		Name:       name,
		ModuleUrl:  DefaultModuleUrl,
		Parameters: parameters,
		Handlers: []topologyConfig.Handler{
			topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: handlers.DefaultHandlerCategory,
					Endpoint: handlerEndpoint,
				},
			},
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topology.ServiceManagerCategory,
				Endpoint: managerEndpoint,
			},
		},
	}
}

func inprocManagerIndependentService(name string, handlerEndpoint message.Endpoint, managerEndpoint message.Endpoint) topologyConfig.Service {
	return topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		Handlers: testHandlers(
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: handlers.DefaultHandlerCategory,
				Endpoint: handlerEndpoint,
			},
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topology.ServiceManagerCategory,
				Endpoint: managerEndpoint,
			},
		),
	}
}
