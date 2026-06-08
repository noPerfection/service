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
	proxyName     = "default-name-proxy"
	proxyCategory = "default-name"
)

func main() {
	app, err := service.NewProxy(proxyName, configPath)
	if err != nil {
		panic(err)
	}

	if err := app.Route(base.Any, onDefaultName, proxyCategory); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("default-name-proxy listening on localhost:8001")

	app.Wait()
}

func onDefaultName(req handlers.ProxyRequest) handlers.ProxyReply {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		req.RouteParameters().Set("name", "Medet Ahmetson")
	}

	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}
