package main

import (
	"fmt"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
)

func main() {
	command := "hello"
	params := datatype.New()
	if arg.FlagExist("age") {
		command = "age-verification"
		params.Set("age", arg.FlagValue("age"))
	} else if arg.FlagExist("name") {
		params.Set("name", arg.FlagValue("name"))
	}

	c, err := client.New("localhost", 8003, client.SyncReplierType)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	c.Timeout(time.Second)
	c.Attempt(1)

	reply, err := c.Request(&message.Request{
		Command:    command,
		Parameters: params,
	})
	if err != nil {
		panic(err)
	}
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	if command == "age-verification" {
		passed, err := reply.ReplyParameters().BoolValue("passed")
		if err != nil {
			panic(err)
		}
		fmt.Println(passed)
		return
	}

	msg, err := reply.ReplyParameters().StringValue("message")
	if err != nil {
		panic(err)
	}
	fmt.Println(msg)
}
