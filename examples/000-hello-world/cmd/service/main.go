package main

import (
	"fmt"

	"github.com/noPerfection/service"
)

func main() {
	app, err := service.New("hello-world", "noPerfection.json")
	if err != nil {
		panic(err)
	}

	err = app.Route("hello", func(req service.RequestInterface) service.ReplyInterface {
		name, err := req.RouteParameters().StringValue("name")
		if err != nil || name == "" {
			name = "world"
		}
		return req.Ok(map[string]any{"message": "hello " + name})
	})
	if err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:8000")
	fmt.Println("run: go run ./cmd/client")

	app.Wait()
}
