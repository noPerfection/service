package proxy

import (
	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
	"strings"
)

const (
	configPath    = "noPerfection.json"
	proxyName     = "default-name-proxy"
	proxyCategory = "main"
)

func New() (*service.Proxy, error) {
	app, err := service.NewProxy(proxyName, configPath)
	if err != nil {
		return nil, err
	}

	if err := app.Route(base.Any, onDefaultName, proxyCategory); err != nil {
		return nil, err
	}

	return app, nil
}

func onDefaultName(req handlers.ProxyRequest) handlers.ProxyReply {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		req.RouteParameters().Set("name", "Medet Ahmetson")
	}
	if strings.Contains(name, "shit") {
		return handlers.ProxyReply{Reply: *req.Fail("I'll tell your mom").(*message.Reply)}
	} else if name == "loser" {
		return handlers.ProxyReply{Reply: *req.Ok(datatype.New().Set("message", "who is loser?")).(*message.Reply)}
	}

	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}