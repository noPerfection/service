package main

import (
	"fmt"
	"os"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
	topologyConfig "github.com/noPerfection/topology/config"
)

const configPath = "noPerfection.json"

func main() {
	slow3000 := arg.FlagExist("slow-3000")
	if !slow3000 {
		if err := resetConfig(); err != nil {
			panic(err)
		}
	}

	app, err := service.New("hardcoded-topology", configPath)
	if err != nil {
		panic(err)
	}

	port := uint64(8000)
	handlerType := topologyConfig.ReplierType
	if slow3000 {
		port = 3000
		handlerType = topologyConfig.SyncReplierType
		if err := app.SetHandlerConfig(topologyConfig.IndependentHandler{
			Type:     handlerType,
			Category: handlers.DefaultHandlerCategory,
			Endpoint: message.NewEndpoint("localhost", port),
		}); err != nil {
			panic(err)
		}
	}

	if err := app.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		code, err := req.RouteParameters().StringValue("code")
		if err != nil || code == "" {
			code = "empty"
		}

		time.Sleep(100 * time.Millisecond)
		return req.Ok(datatype.New().Set("message", "hello "+code))
	}); err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Printf("hardcoded-topology service listening on localhost:%d as %s\n", port, handlerType)
	fmt.Println("run: go run ./cmd/client --port=" + fmt.Sprint(port))

	app.Wait()
}

func resetConfig() error {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return os.Remove(configPath)
}
