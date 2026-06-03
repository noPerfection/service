package main

import (
	"fmt"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/protocol/client"
	managerClient "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/manager"
)

func main() {
	if arg.FlagExist("close") {
		closeService()
		return
	}

	callHello()
}

func callHello() {
	c, err := client.New("localhost", 8000, client.ReplierType)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	c.Timeout(time.Second)
	c.Attempt(1)

	reply, err := c.Request(&message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	})
	if err != nil {
		panic(err)
	}
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	message, err := reply.ReplyParameters().StringValue("message")
	if err != nil {
		panic(err)
	}
	fmt.Println(message)
}

func closeService() {
	c, err := managerClient.NewClient("localhost", 8001)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	reply, err := c.Request(&message.Request{
		Command:    manager.StopService,
		Parameters: datatype.New().Set("service", "close-service"),
	})
	if err != nil {
		panic(err)
	}
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	fmt.Println("close signal sent")
}
