package entrypoint

import (
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	npservice "github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	ServiceName        = "entrypoint"
	Category           = "main"
	ServiceManagerPort = 8005
)

func New(configPath string) (*npservice.Proxy, error) {
	app, err := npservice.NewProxy(ServiceName)
	if err != nil {
		return nil, err
	}
	if configPath != "" {
		if err := app.SetTopologyParams(map[string]any{npservice.TopologyParamFilepath: configPath}); err != nil {
			return nil, err
		}
	}

	if err := app.SetEndpoint(message.NewEndpoint("localhost", ServiceManagerPort), topologyConfig.ServiceManagerCategory); err != nil {
		return nil, err
	}

	if err := app.Route(base.Any, onForward, Category); err != nil {
		return nil, err
	}

	return app, nil
}

func onForward(req handlers.ProxyRequest) handlers.ProxyReply {
	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}
