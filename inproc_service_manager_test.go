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
			inprocServices := 0
			err := (&Independent{}).validateInprocServiceManagersFor(tt.service, &inprocServices)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
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
			topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
				IndependentHandler: topologyConfig.IndependentHandler{
					Type:     topologyConfig.SyncReplierType,
					Category: handlers.DefaultHandlerCategory,
					Endpoint: handlerEndpoint,
				},
			}),
			topologyConfig.NewHandlerVariant(topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topology.ServiceManagerCategory,
				Endpoint: managerEndpoint,
			}),
		},
	}
}

func inprocManagerIndependentService(name string, handlerEndpoint message.Endpoint, managerEndpoint message.Endpoint) topologyConfig.Service {
	return topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      name,
		ModuleUrl: DefaultModuleUrl,
		Handlers: topologyConfig.NewHandlerVariants(
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
