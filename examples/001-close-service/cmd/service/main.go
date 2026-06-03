package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
)

func main() {
	managerEndpoint := message.NewEndpoint("localhost", 8001)
	app, err := service.New("close-service", "noPerfection.json", managerEndpoint)
	if err != nil {
		panic(err)
	}

	replyWorld := func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New().Set("message", "hello and world"))
	}

	if err := app.Route("hello", replyWorld); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello service is running")
	fmt.Println("run: go run ./cmd/client")
	fmt.Println("stop: go run ./cmd/client --close")

	app.Wait()
}
