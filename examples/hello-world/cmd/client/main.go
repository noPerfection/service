package main

import (
	"fmt"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
)

func main() {
	c, err := client.New("localhost", 8000, client.ReplierType)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	c.Timeout(time.Second)
	c.Attempt(1)

	reply, err := c.Request(&message.Request{
		Command:    "hello",
		Parameters: datatype.New().Set("name", "independent"),
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
