package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/topology"
)

const serviceManagerPort = 8001

func main() {
	app, err := service.New("close-service")
	if err != nil {
		panic(err)
	}

	if err := app.SetEndpoint(message.NewEndpoint("localhost", serviceManagerPort), topology.ServiceManagerCategory); err != nil {
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
