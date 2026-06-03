# Service
*This is one of the core modules.*

The *service* library allows creating of independent micro**services**.

> We omit the **micro** prefix from now on.

**The independent services are minimal stand-alone applications.**
A developer defines the list of API routes and a function that's executed.

The API routes are grouped into the [handlers](https://github.com/noPerfection/protocol/handler).
*The handler defines how the API is served to the external users.*

The independent services are isolated from each other.
They don't share a configuration nor the data that they are working on.

## Application architecture

The application consists of multiple services.

The independent service is the core of the application.
As it will keep the business logic.

There are auxiliary services.

The proxies that operate on a request before it's passed to a service.
And extensions extending service possibility or do some side works.

**One of the aims of noPerfection framework is to write self-orchestrating applications.**

In the noPerfection framework, the services are organized in a parent-child relationship.
The independent service acts as a root node. 
The auxiliary services counted as the child nodes.

That means the root node is responsible for spawning the children.

It's done in the background by the framework, so developers don't have to worry about it.

## Coarsely grained API

When you define an API routes, you set them at the minimal level.
But if the service is stored on another machine, then requesting data by minimal API will cause a delay.

In the microservice architecture, the remote APIs must be coarsely grained to reduce the network hops.

The extensions with *merge* flag can group the routes of the parent.

# LifeCycle
When a service runs, it prepares itself.
The preparation is composed of two steps.
First, it creates a configuration based on the required parameters.
Then, it fills them. 
If a user passed a pre-defined configuration,
then the process will validate and apply them.
If there is no configuration, then it will create a random configuration.
That random configuration is written to the path.

When the service is prepared, it runs the manager. 
The manager is set in the *"PREPARED"* state. 
If the service has a parent service, then it will send the message to the parent.

After running the manager, the service runs the dependency manager.
The dependency manager running means three things.
If the service is running, it will ask to acknowledge the parent.
Acknowledging the means to ask permission to connect from the parent.
If the service is not acknowledged, it will mark that service as failed.
If the service is not running, it will check the binary.
If the binary exists, the service will run that binary.
If the binary does not exist, the dependency manager will install it.
Then run it.
The dependency manager is working with the nearby proxies and extensions.

> As a parent, it passes an id, configuration and its own id as a parent.

When the dependencies are all set, it updates the state to *"READY"*.
When some dependencies are not set, it will mark itself as *"PARTIALLY_READY"*.

When it's (partially) ready, the dependency manager creates a heartbeat tracker.
This tracker is given to the manager.
Then, the manager creates its own heartbeat, and sends the messages to the parent.
If no parent is given, it won't set it.

# Hello World

Install the module

```bash
go get github.com/noPerfection/service
```

```go
package main

import (
	"fmt"

	"github.com/noPerfection/datatype" // loaded indirectly with service module
	"github.com/noPerfection/protocol/message" // loaded indirectly with service module
	"github.com/noPerfection/service"
)

func main() {
	app, _ := service.New()

	replyWorld := func(req message.RequestInterface) message.ReplyInterface {
		name, _ := req.RouteParameters().StringValue("name")
		if name == "" {
			name = "world"
		}
		return req.Ok(datatype.New().Set("message", "hello "+name))
	}

	app.Route("hello", replyWorld)

	app.Start()
	defer app.Stop()

	fmt.Println("hello service is running")
	app.Wait()
}
```

`Route` must be called before `Start`. If no handler category is passed, the route is attached to the default `main` handler. When no `main` handler is configured, the service creates one on `localhost:8000`.

See [examples/hello-world](./examples/hello-world) for a live example that you can run locally.

You can call the route with a protocol client:

```go
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
		Parameters: datatype.New().Set("name", "world"),
	})
	if err != nil {
		panic(err)
	}
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	msg, _ := reply.ReplyParameters().StringValue("message")
	fmt.Println(msg)
}
```

## Why noPerfection

The service comes with a built-in admin panel. You can manage it from other parts of the system by restarting a microservice, stopping it, or closing one of its handler threads.

To do that, each service starts a manager. The manager is internal by default, so you can use it from code. You can also expose it on a known endpoint and send a signal to the service to stop itself.

See [examples/001-close-service](./examples/001-close-service) for the runnable example.

Following the Hello World example, use a custom manager endpoint:

```go
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
	app.Wait()
}
```

Then create a client that uses `github.com/noPerfection/os/arg` for flags. Without flags, it calls the `hello` route. With `--close`, it sends a manager command that stops the service.

```go
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

	message, _ := reply.ReplyParameters().StringValue("message")
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
```

Run the service in one terminal, then run the client in another:

```bash
go run ./cmd/client
go run ./cmd/client --close
```

After `--close`, look back at the service terminal. The service should stop because the manager releases `app.Wait()`.

## Tutorial 3: Override the Handler Configuration

By default, when no handler config is provided, an independent service creates a
`main` handler using:

- handler type: `Replier`
- endpoint: `localhost:8000`

You can overwrite that handler configuration from code before the service starts.
This is useful when you want the same Hello World service logic, but served by a
different handler type or port.

In this example, the `main` handler is changed to:

- handler type: `SyncReplier`
- endpoint: `localhost:3000`

```go
package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	topologyConfig "github.com/noPerfection/topology/config"
)

func main() {
	app, err := service.New("hello-world", "noPerfection.json")
	if err != nil {
		panic(err)
	}

	// Define the config for the default "main" handler.
	if err := app.SetHandlerConfig(topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: "main",
		Endpoint: message.NewEndpoint("localhost", 3000),
	}); err != nil {
		panic(err)
	}

	// Other app logic and routings.
	err = app.Route("hello", func(req message.RequestInterface) message.ReplyInterface {
		name, err := req.RouteParameters().StringValue("name")
		if err != nil || name == "" {
			name = "world"
		}
		return req.Ok(datatype.New().Set("message", "hello "+name))
	})
	if err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:3000")
	app.Wait()
}
```

Because the handler type changed, the client should connect as a sync replier:

```go
c, err := client.New("localhost", 3000, client.SyncReplierType)
```


# noPerfection Service

*noPerfection Service* is a library to create services on **noPerfection**. 

**noPerfection** is the protocol, libraries, and an account system.
The aim of it to create meaningful services and also useful for other developers.

The *account system* consists of the API service authentication and payment system.

## Contents

* [noPerfection Service](#noperfection-service)
* [Contents](#contents)
* [Components](#components)
* * [Service](#service)
* * * [Independent](#independent)
* * * [Extension](#extension)
* * * [Proxy](#proxy)
* * [Controller](#controller)
* * * [Replier](#replier)
* * * [Publisher](#publisher)
* * * [Puller](#puller)
* * * [Router](#router)
* * [Configuration](#configuration)
* * * [Env](#environment-variables)
* * * [Yaml](#yaml)
* [Flags](#service-flags)
* [Further Reading](#further-reading)

---

## Components

## Service
A **service** is a solution for a one problem as an independent
software. An **app** is an interconnection of the services. 

> Since services are independent software, then, an **app** 
will be considered as a distributed system.

> Single service itself also acts as an **app**. Here, we just refer as **app**
> to the specific business case of your need.



The services are created using **noPerfection Service**. 
The goal of **noPerfection Service** is to write re-usable solutions that will be
useful to another project with a minimal setup. Hence, why the services are
standalone applications .

To compose an app from the services in a structured way, the services are
divided into three categories.

### Independent
The first type of the services are **independent** services. Read it
as an independent software. Your app should have one independent service
that keeps the core logic of your application.

Independent services will rarely be shared. So the source
code could be private.

### Extension
The second type of the services are **extension** services. The extensions
are the solutions that could be re-used by multiple projects.

This is the core part that all makes the services as re-usable.

The extensions are allowed to be connected from the independent services.
And doesn't work with the users directly.

### Proxy
The third and last type of the services are **proxy** services. The proxy
acts as a switch between a user/service and a user/service. Depending on 
the proxy result the request will be forwarded or returned back to the client.

---
## Controller
Since the services are the units of distributed system, services
has to talk to each other. And services has to talk with the external world.

Therefore, each service acts as a server. The service mechanism 
transfers in or out some messages. 
This mechanism is implemented through controllers.

> Controller is an alias of server.
> 
> Controller term comes from the MPC pattern.

A service may have multiple controllers, at least one. 
The controllers receive the messages. Then controller is routing
the messages to the handlers. To find the right handler, the messages 
include the commands.

For optimization needs, there are different kinds of controllers.

### Replier
A **replier** controller handles a one request at a time. All incoming
requests are queued internally, until the current request is not executed.
When the request is executed, the controller returns the status to the callee.
Then replier will execute the next request in the queue.

> The requester will be waiting the response of the controller

### Router
A **router** controller handles many requests at a time. Upon the execution,
the router will reply the status back to the callee.

> The requester will be waiting the response of the controller

### Puller
A **puller** controller handles a one request at a time. All requests will be
queued internally. When the controller finishes the execution, it will
execute the next request in the queue.

Puller will not respond back to the callee about the status.

> The requester will not wait for the response of the controller.
> So this one is faster.

### Publisher
A **publisher** controller sends messages to the subscribers. It doesn't
receive the request from outside. But has internal **puller** controller
that the **publisher** is connected too. Any message coming into the **puller**
invokes mass message broadcast by the **publisher**.

> The subscriber waits for the controller, but doesn't request to the publisher.
> The invoker of the puller doesn't wait for the response of the publisher.

---

## Configuration
Any apps created by this module is loading environment
variables by default.

As well as it requires the *configuration* in yaml format.

### Env

You can set the Yaml file name as well as it's path
using the following environment variables:
```bash
SERVICE_CONFIG_NAME=service
SERVICE_CONFIG_PATH=.
```

By default, the service will look for `service.yml` in the `.` directory.

### Yaml

The configuration format is this:
```yaml
Services:
  - Type: Independent # or Proxy | Extension
    Name: 
    Instance: 
    Controllers:
      - Type: Replier # or Puller | Publisher | Router
        Name: "myApi"
        Instances:
          - Port: 2302
            Instance: 
    Proxies:
      - Name: "auth"
        Port: 8000

    Pipelines:
      - "auth->myApi"

    Extensions:
      - Name: "database"
        Port: 8002
```

At root, it has `Services` with at least one Service defined.
The service has the following parameters:

* Type which defines what kind of service it is. It could be `Independent`, `Proxy` or `Extension`.
* Name of the service. If you define multiple services, then their Type and Name should match.
* Instance is the unique identifier of this service. If you have multiple services, then it should have different instance.
* Controllers lists what kind of command handlers it has.
* Proxies lists what kind of proxies it has.
* Pipelines should have one or more proxy pipeline. The last name should name of the controller instance.
* Extensions lists the extensions that this service depends on. All these extensions are passed to the controllers.

The **controllers** are the command handlers. All incoming requests
from the users (whether it's through proxy or not) are handled by the controllers.
The parameters of the controllers:
* Type which defines what kind of controller it is. It could be Replier, Puller, Publisher or Router.
* Name of the controller to classify it.
* Instances describes the unique controllers of this type.

The controller instances have the following parameters:
* Instance is the unique id of the controller within all service
* Port where the controller exposes itself

Proxy has the following parameters:
* Name of the proxy
* Port where the proxy set too.

Extension has the following parameters:
* Name of the extension
* Port where the extension set too.

---

## Flags

The built service will have several flags and arguments.

* `--secure` enables authentication
* `--generate-config` rather than starting, it will create a default yaml configuration
* `--path=./a.yml` relative path of the generated configuration. The value should end with *.yml*.
* `--configuration=./b.yml` relative path of the configuration to use.

---

## Further reading

Once you understand the Services, go read further to create different kinds of services.

* [Proxy](./PROXY.md) to define proxy services.
* [Extension](./EXTENSION.md) to define extension services.
* [Independent](./INDEPENDENT.md) to define independent services.
