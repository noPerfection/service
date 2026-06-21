package main

import (
	"fmt"

	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
)

const (
	configPath    = "noPerfection.json"
	proxyName     = "entrypoint"
	proxyCategory = "main"
)

func main() {
	app, err := service.NewProxy(proxyName, configPath)
	if err != nil {
		panic(err)
	}

	if err := app.Route(base.Any, onForward, proxyCategory); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("entrypoint proxy listening on localhost:8003")

	app.Wait()
}

func onForward(req handlers.ProxyRequest) handlers.ProxyReply {
	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}
