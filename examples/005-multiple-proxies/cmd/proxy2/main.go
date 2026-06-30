package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
)

const (
	configPath    = "noPerfection.json"
	proxyName     = "upper-case-names"
	proxyCategory = "main"
)

func main() {
	app, err := service.NewProxy(proxyName)
	if err != nil {
		panic(err)
	}

	if err := app.Route(base.Any, onUpperCaseNames, proxyCategory); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("upper-case-names proxy listening on localhost:8003")

	app.Wait()
}

func onUpperCaseNames(req handlers.ProxyRequest) handlers.ProxyReply {
	name, err := req.RouteParameters().StringValue("name")
	if err == nil && name != "" {
		req.RouteParameters().Set("name", normalizeName(name))
	}

	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}

func normalizeName(name string) string {
	parts := strings.Fields(name)
	for i, part := range parts {
		letters := []rune(strings.ToLower(part))
		if len(letters) == 0 {
			continue
		}
		letters[0] = unicode.ToUpper(letters[0])
		parts[i] = string(letters)
	}
	return strings.Join(parts, " ")
}
