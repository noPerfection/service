package entrypoint

import (
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
)

const (
	configPath     = "noPerfection.json"
	entrypointName = "entrypoint"
	proxyCategory  = "main"
)

func New() (*service.Proxy, error) {
	app, err := service.NewProxy(entrypointName)
	if err != nil {
		return nil, err
	}

	if err := app.Route(message.Any, onForward, proxyCategory); err != nil {
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
