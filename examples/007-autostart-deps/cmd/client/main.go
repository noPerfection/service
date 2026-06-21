package main

import (
	"fmt"
	"os"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/protocol/client"
	managerClient "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/manager"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	entrypointEndpoint = "tmp/entrypoint_proxy"
	serviceManagerPort = 8001
)

func main() {
	if arg.FlagExist("help") {
		printHelp()
		return
	}
	if arg.FlagExist("services") {
		listServices()
		return
	}
	if serviceName := arg.FlagValue("stop"); serviceName != "" {
		managerLifecycle(manager.StopService, serviceName)
		return
	}
	if serviceName := arg.FlagValue("start"); serviceName != "" {
		managerLifecycle(manager.StartService, serviceName)
		return
	}
	if serviceName := arg.FlagValue("status"); serviceName != "" {
		printServiceStatus(serviceName)
		return
	}

	callEntrypoint()
}

func printHelp() {
	fmt.Println(`Usage:
  go run ./cmd/client
  go run ./cmd/client --name="Medet Ahmetson"
  go run ./cmd/client --age=21
  go run ./cmd/client --services
  go run ./cmd/client --status=<service-name>
  go run ./cmd/client --start=<service-name>
  go run ./cmd/client --stop=<service-name>
  go run ./cmd/client --help`)
}

func newManagerClient() (*managerClient.Client, error) {
	client, err := managerClient.NewClient("localhost", serviceManagerPort)
	if err != nil {
		return nil, err
	}
	client.Timeout(time.Second * 30)
	client.Attempt(2)
	return client, nil
}

func listServices() {
	c, err := newManagerClient()
	if err != nil {
		panic(err)
	}
	defer c.Close()

	reply, err := c.Request(&message.Request{
		Command:    manager.Services,
		Parameters: datatype.New(),
	})
	if err != nil {
		panic(err)
	}
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	rawServices, err := reply.ReplyParameters().NestedListValue("services")
	if err != nil {
		panic(err)
	}

	for _, rawService := range rawServices {
		var service topologyConfig.Service
		if err := rawService.Interface(&service); err != nil {
			panic(err)
		}
		running, err := serviceRunning(c, service.Name)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s running=%t\n", service.Name, running)
	}
}

func serviceRunning(c *managerClient.Client, serviceName string) (bool, error) {
	reply, err := c.Request(&message.Request{
		Command:    manager.IsServiceRunning,
		Parameters: datatype.New().Set("service", serviceName),
	})
	if err != nil {
		return false, err
	}
	if !reply.IsOK() {
		return false, fmt.Errorf("%s", reply.ErrorMessage())
	}
	return reply.ReplyParameters().BoolValue("running")
}

func printServiceStatus(serviceName string) {
	c, err := newManagerClient()
	if err != nil {
		panic(err)
	}
	defer c.Close()

	running, err := serviceRunning(c, serviceName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s running=%t\n", serviceName, running)
}

func managerLifecycle(command string, serviceName string) {
	c, err := newManagerClient()
	if err != nil {
		panic(err)
	}
	defer c.Close()

	reply, err := c.Request(&message.Request{
		Command:    command,
		Parameters: datatype.New().Set("service", serviceName),
	})
	if err != nil {
		panic(err)
	}
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	switch command {
	case manager.StartService:
		id, err := reply.ReplyParameters().StringValue("id")
		if err != nil {
			panic(err)
		}
		if id == "" {
			fmt.Printf("%s is already running\n", serviceName)
			return
		}
		fmt.Printf("started %s id=%s\n", serviceName, id)
	case manager.StopService:
		fmt.Printf("stopped %s\n", serviceName)
	default:
		fmt.Printf("%s completed for %s\n", command, serviceName)
	}
}

func callEntrypoint() {
	command := "hello"
	params := datatype.New()
	if arg.FlagExist("age") {
		command = "age-verification"
		params.Set("age", arg.FlagValue("age"))
	} else if arg.FlagExist("name") {
		params.Set("name", arg.FlagValue("name"))
	}

	c, err := client.New(entrypointEndpoint, 0, client.SyncReplierType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
