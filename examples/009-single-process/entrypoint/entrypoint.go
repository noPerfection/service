package entrypoint

import (
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	npservice "github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
)

const (
	ServiceName = "entrypoint"
	Category    = "main"
)

func New(configPath string) (*npservice.Proxy, error) {
	app, err := npservice.NewProxy(ServiceName, configPath, message.NewEndpoint("localhost", 8005))
	if err != nil {
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
