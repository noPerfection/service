package metrics

import (
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
)

const (
	proxyName     = "metrics"
	proxyCategory = "main"
)

func New() (*service.Proxy, error) {
	logger, err := log.New(proxyName, true)
	if err != nil {
		return nil, err
	}

	app, err := service.NewProxy(proxyName)
	if err != nil {
		return nil, err
	}

	if err := app.Route(base.Any, onMetrics(logger), proxyCategory); err != nil {
		return nil, err
	}

	return app, nil
}

func onMetrics(logger *log.Logger) handlers.ProxyHandleFunc {
	return func(req handlers.ProxyRequest) handlers.ProxyReply {
		logger.Warn("request", "command", req.CommandName())

		reply, err := req.Forward()
		if err != nil {
			logger.Error("forward failed", "command", req.CommandName(), "err", err)
			return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
		}

		if reply.IsOK() {
			logger.Error("reply", "command", req.CommandName(), "status", "ok")
		} else {
			logger.Info("reply", "command", req.CommandName(), "status", "error", "message", reply.ErrorMessage())
		}

		return reply
	}
}
