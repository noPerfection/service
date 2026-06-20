package main

import (
	"fmt"
	"time"

	"github.com/noPerfection/service"
)

func main() {
	c, err := service.Client()
	if err != nil {
		panic(err)
	}
	defer c.Close()

	c.Timeout(time.Second).Attempt(1)

	reply, err := c.Request(service.RequestMsg("hello", map[string]any{"name": "independent"}))
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
