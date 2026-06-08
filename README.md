# noPerfection/service
Use this go module to turn your go app, package into a noPerfect microservice.

> We omit the **micro** prefix from now on.

Service at its core a collection of [zeromq](https://zeromq.org/) sockets called [handlers](https://github.com/noPerfection/protocol/handler).

# Hello World

Install the module

```bash
go get github.com/noPerfection/service
```

Write the main file at `cmd/server/main.go`
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

	app.Route("hello", onHello)

	app.Start()
	defer app.Close()

	fmt.Println("hello service is running")
	app.Wait()
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, _ := req.RouteParameters().StringValue("name")

	return req.Ok(datatype.New().Set("message", "hello "+name))
}
```

Then, launch it `go run ./cmd/service/main.go`, it should be running.

It's time to test by sending our name. Lets create a new app at
`cmd/client/main.go`

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
	c, _ := client.New("localhost", 8000, client.ReplierType)

	req := message.Request{
		Command:    "hello",
		Parameters: datatype.New().Set("name", "Jonny Dough"),
	}

	reply, _ := c.Request(&req)

	msg, _ := reply.ReplyParameters().StringValue("message")
	fmt.Println(msg)
}
```

if we launch our app on a new terminal tab we will see the greetings:
`go run ./cmd/client/main.go`.

See [examples/hello-world](./examples/hello-world) source code.

## Why noPerfection

That's so far simple, but doesn't tell you its advantage.
The service comes with a built-in admin panel thats available on a different port. 

You can manage it from other parts of the system by restarting a microservice, stopping it, or closing one of its handler threads.

To do that, each service starts a manager. The manager is internal by default, available within a code.
But lets change the manager's endpoint.

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
	// Add the endpoint exposing on port 8001
	managerEndpoint := message.NewEndpoint("localhost", 8001)
	app, _ := service.New("close-service", "noPerfection.json", managerEndpoint)

	app.Route("hello", onHello)

	app.Start()
	defer app.Close()

	app.Wait()
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	return req.Ok(datatype.New().Set("message", "hello and world"))
}
```

Then, create app will create two sockets.
One to connect to the server, while second connects to the manager of service.
I'll use `github.com/noPerfection/os/arg` package to add `--close` flag. Without it,
client requests `hello`. With `--close`, it sends a signal to manager.

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
	c, _ := client.New("localhost", 8000, client.ReplierType)
	defer c.Close()

	c.Timeout(time.Second)
	c.Attempt(1)

	reply, err := c.Request(&message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	})
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	message, _ := reply.ReplyParameters().StringValue("message")
	fmt.Println(message)
}

func closeService() {
	c, _ := managerClient.NewClient("localhost", 8001)
	defer c.Close()

	reply, err := c.Request(&message.Request{
		Command:    manager.StopService,
		Parameters: datatype.New().Set("service", "close-service"),
	})
	if !reply.IsOK() {
		panic(reply.ErrorMessage())
	}

	fmt.Println("Service was closed")
}
```

Run the service in one terminal, then run the client in another:

```bash
go run ./cmd/client
go run ./cmd/client --close
```

After `--close`, look back at the service terminal. The service should stop because the manager releases `app.Wait()`.

See [examples/001-close-service](./examples/001-close-service) for the runnable example.

## Tutorial 3: Custom handlers

By default, a service creates a `main` handler that is a classic server backend.
It's run on the `tcp://localhost:8000` path.

We can overwrite that handler before service starts.
We can change the way how server replies by changing the socket type.

Let's change `main` handler to:

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
	handlerConf := topologyConfig.Handler{
		Type:     topologyConfig.SyncReplierType,
		Category: "main",
		Endpoint: message.NewEndpoint("localhost", 3000),
	}

	app, err := service.New()

	// Define the config for the default "main" handler.
	app.SetHandlerConfig(handlerConf)

	// Other app logic and routings.
	err = app.Route("hello", onHello)

	app.Start()
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:3000")
	app.Wait()
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, _ := req.RouteParameters().StringValue("name")
	return req.Ok(datatype.New().Set("message", "hello "+name))
}
```

Because the handler type changed, the client should connect as a sync replier:

```go
c, err := client.New("localhost", 3000, client.SyncReplierType)
```

SyncReplier handles one request at a time. All requests are queued, and until service
doesn't handle the current code, it will keep others in idle mode.
There are other types of the handlers such as **Publisher**, **Worker**, **Pair** as well.

---

Just like handlers, you can define service configs from code too. This lets you
create a service config before startup, or overwrite parameters that would
otherwise come from `noPerfection.json`.

```go
app.SetServiceConfig(topologyConfig.Service{
	Type:      topologyConfig.IndependentType,
	Name:      "hello-world",
	ModuleUrl: "github.com/noPerfection/service",
	Parameters: datatype.New().Set("mode", "tutorial"),
})
```

Call `SetServiceConfig` before `Start()`. Hardcoded handler configs are applied
after hardcoded service configs, so handlers can be attached to services that
were created in code.

If no services defined by a programmer, then service will create a default one with
the name `main`.

## Tutorial 4: Default name proxy

Let's now create a proxy.

We already have a `hello-world` service that receives the `name` parameter.
The client will pass `--name=<name>`. When the flag is not given, or it is empty,
we want to use the proxy's default name.

Why not change the service and put the default name there? Because the service
should stay focused on the business rule: `hello` requires a name. Defaulting a
client input is proxy behavior. Keeping it in the proxy keeps the service clean
and avoids adding client-specific details to the service logic.

First, create the service at `cmd/service/main.go`. It defines the proxy with
`SetServiceConfig`, and the proxy handler forwards to the service's default
`main` handler. The proxy listens on `localhost:8001`.

```go
package main

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	configPath    = "noPerfection.json"
	serviceName   = "hello-world"
	proxyName     = "default-name-proxy"
	proxyCategory = "default-name"
)

func main() {
	app, err := service.New(serviceName, configPath)
	if err != nil {
		panic(err)
	}

	err = app.SetServiceConfig(topologyConfig.Service{
		Type:      topologyConfig.ProxyType,
		Name:      proxyName,
		ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy",
		Handlers: []topologyConfig.HandlerVariant{
			topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
				Handler: topologyConfig.Handler{
					Type:     topologyConfig.SyncReplierType,
					Category: proxyCategory,
					Endpoint: message.NewEndpoint("localhost", 8001),
				},
				Routes: []string{base.Any},
				Outbounds: []topologyConfig.ServicePointer{
					topologyConfig.ServiceTarget(topologyConfig.Service{
						Type:      topologyConfig.IndependentType,
						Name:      serviceName,
						ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy",
						Handlers: topologyConfig.NewHandlerVariants(topologyConfig.Handler{
							Type:     topologyConfig.ReplierType,
							Category: handlers.DefaultHandlerCategory,
							Endpoint: message.NewEndpoint("localhost", 8000),
						}),
					}),
				},
			}),
		},
	})
	if err != nil {
		panic(err)
	}

	err = app.Route("hello", onHello)
	if err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("hello-world service listening on localhost:8000")
	app.Wait()
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		return req.Fail("name is required")
	}

	return req.Ok(datatype.New().Set("message", "hello "+name))
}
```

Then create the proxy at `cmd/proxy/main.go`. It handles `base.Any`, sets
`name` to `Medet Ahmetson` when the value is missing or empty, and forwards the
request to the service.

```go
package main

import (
	"fmt"

	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/handlers"
)

func main() {
	app, err := service.NewProxy("default-name-proxy", "noPerfection.json")
	if err != nil {
		panic(err)
	}

	err = app.Route(base.Any, onDefaultName, "default-name")
	if err != nil {
		panic(err)
	}

	app.Start()
	defer app.Stop()

	fmt.Println("default-name-proxy listening on localhost:8001")
	app.Wait()
}

func onDefaultName(req handlers.ProxyRequest) handlers.ProxyReply {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		req.RouteParameters().Set("name", "Medet Ahmetson")
	}

	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}
```

Finally, create the client at `cmd/client/main.go`. The client accepts
`--name=<name>`, but it connects to the proxy on port `8001` instead of calling
the service directly.

```go
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
	params := datatype.New()
	if arg.FlagExist("name") {
		params.Set("name", arg.FlagValue("name"))
	}

	c, err := client.New("localhost", 8001, client.SyncReplierType)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	c.Timeout(time.Second)
	c.Attempt(1)

	reply, err := c.Request(&message.Request{
		Command:    "hello",
		Parameters: params,
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

Run the service, proxy, and client in separate terminals:

```bash
go run ./cmd/service
go run ./cmd/proxy
go run ./cmd/client --name="Jonny Dough"
go run ./cmd/client
```

The first client call prints `hello Jonny Dough`. The second call omits the
name, so the proxy fills it and the service returns `hello Medet Ahmetson`.

See [examples/004-default-name-proxy](./examples/004-default-name-proxy) for the
runnable example.

## Tutorial 5: Proxy by command deps

In [Tutorial 4](#tutorial-4-default-name-proxy), we created a proxy handler with
an outbound parameter pointing to the `hello-world` service. That works, but
managing endpoints and keeping those outbounds synchronized is tiresome.

Instead, we can make it dynamic with `SetCommandDeps`.

The proxy and client do not change. They stay the same as
`cmd/proxy/main.go` and `cmd/client/main.go` from Tutorial 4. The service is
almost identical too, but its topology is more fine grained.

First, define the proxy service. It has a proxy handler on `localhost:8001`, but
no outbound is written by hand:

```go
app.SetServiceConfig(topologyConfig.Service{
	Type:      topologyConfig.ProxyType,
	Name:      proxyName,
	ModuleUrl: "github.com/noPerfection/service/examples/005-command-deps/cmd/proxy",
	Handlers: []topologyConfig.HandlerVariant{
		topologyConfig.NewProxyHandlerVariant(topologyConfig.ProxyHandler{
			Handler: topologyConfig.Handler{
				Type:     topologyConfig.SyncReplierType,
				Category: proxyCategory,
				Endpoint: message.NewEndpoint("localhost", 8001),
			},
		}),
	},
})
```

Then define the proxy manager endpoint. The service uses this endpoint to
synchronize the command dependency with the proxy process:

```go
app.SetHandlerConfig(topologyConfig.Handler{
	Type:     topologyConfig.SyncReplierType,
	Category: topology.ServiceManagerCategory,
	Endpoint: message.NewEndpoint("localhost", 8002),
}, proxyName)
```

Finally, declare that the `hello` command should go through the proxy:

```go
app.SetCommandDeps(topologyConfig.DepService{
	Name: "hello",
	Proxies: []topologyConfig.ServicePointer{
		topologyConfig.RefTarget(proxyName),
	},
})
```

Now the service can synchronize the proxy handler route and outbound for the
`hello` command. Run the service, proxy, and client in separate terminals:

```bash
go run ./cmd/service
go run ./cmd/proxy
go run ./cmd/client --name="Jonny Dough"
go run ./cmd/client
```

The first client call prints `hello Jonny Dough`. The second call omits the
name, so the proxy fills it and the service returns `hello Medet Ahmetson`.

See [examples/005-command-deps](./examples/005-command-deps) for the full
example.

## Tutorial 6: Multiple proxies together

Command deps can also pipe several proxies together. The order is top to bottom:
the first proxy in `Proxies` is the proxy the user sees, and each proxy forwards
to the next one. The last proxy forwards to this service.

Suppose names always need to be initialized after a name exists:

* `MEDET` becomes `Medet`
* `MEDeT  aHMETSON` becomes `Medet Ahmetson`

We can keep the first proxy from Tutorial 5. It still fills a missing name. Then
we add a second proxy named `upper-case-names` in `cmd/proxy2/main.go`. That
proxy normalizes the name and forwards the request to the service.

The client and first proxy stay the same as
`examples/005-command-deps/cmd/client/main.go` and
`examples/005-command-deps/cmd/proxy/main.go`.

The service only needs to configure both proxy services, their main handlers,
and their managers:

```go
app.SetServiceConfig(proxyConfig(defaultProxyName, defaultProxyPackage, 8001))
app.SetHandlerConfig(proxyManagerConfig(8002), defaultProxyName)

app.SetServiceConfig(proxyConfig(formatProxyName, formatProxyPackage, 8003))
app.SetHandlerConfig(proxyManagerConfig(8004), formatProxyName)
```

Then declare the `hello` command proxy chain:

```go
app.SetCommandDeps(topologyConfig.DepService{
	Name: "hello",
	Proxies: []topologyConfig.ServicePointer{
		topologyConfig.RefTarget(defaultProxyName),
		topologyConfig.RefTarget(formatProxyName),
	},
})
```

With this order, requests flow like this:

```text
client -> default-name-proxy -> upper-case-names -> hello-world
```

Run the service and both proxies in separate terminals:

```bash
go run ./cmd/service
go run ./cmd/proxy
go run ./cmd/proxy2
go run ./cmd/client --name="MEDeT  aHMETSON"
go run ./cmd/client
```

Both client calls print `hello Medet Ahmetson`. The first call formats the
provided name. The second call fills the default name first, then formats it.

See [examples/006-multiple-proxies](./examples/006-multiple-proxies) for the
full example.

## Tutorial 7: Proxy by handler deps

Command deps are useful when a single command needs a proxy. But sometimes the
client should not need to know which socket belongs to which command.

Suppose our service has two routes:

* `hello`
* `age-verification`

The `hello` command should still use the default-name proxy from Tutorial 5.
The `age-verification` command simply receives an integer `age` and returns a
`passed` boolean. It does not need the default-name proxy.

Without handler deps, the client would need to know two sockets: one for the
default-name proxy and one for the service. That is tiresome. Instead, we add an
`entrypoint` proxy. The client calls only this entrypoint socket, and internally
the entrypoint forwards each command to the right next hop.

First, configure both proxy services and their managers:

```go
app.SetServiceConfig(proxyConfig(defaultProxyName, defaultProxyPackage, 8001))
app.SetHandlerConfig(proxyManagerConfig(8002), defaultProxyName)

app.SetServiceConfig(proxyConfig(entrypointName, entrypointPackage, 8003))
app.SetHandlerConfig(proxyManagerConfig(8004), entrypointName)
```

Then keep the command dep for `hello`:

```go
app.SetCommandDeps(topologyConfig.DepService{
	Name: "hello",
	Proxies: []topologyConfig.ServicePointer{
		topologyConfig.RefTarget(defaultProxyName),
	},
})
```

Finally, set the entrypoint as a handler dep for the service handler:

```go
app.SetHandlerDeps(topologyConfig.DepService{
	Name: handlers.DefaultHandlerCategory,
	Proxies: []topologyConfig.ServicePointer{
		topologyConfig.RefTarget(entrypointName),
	},
})
```

The entrypoint code is intentionally small. It just forwards whatever command it
receives:

```go
func onForward(req handlers.ProxyRequest) handlers.ProxyReply {
	reply, err := req.Forward()
	if err != nil {
		return handlers.ProxyReply{Reply: *req.Fail(err.Error()).(*message.Reply)}
	}

	return reply
}
```

During startup, handler-dep sync whitelists the service handler route commands
on the entrypoint. For commands that also have command deps, it configures a
`forward` entry to the first command proxy. So `hello` goes through
`default-name-proxy`, while `age-verification` goes directly to the service.

Run the service, default-name proxy, entrypoint, and client:

```bash
go run ./cmd/service
go run ./cmd/proxy
go run ./cmd/entrypoint
go run ./cmd/client
go run ./cmd/client --age=21
```

The first client call prints `hello Medet Ahmetson`. The second prints `true`.
Both calls use the same entrypoint socket.

See [examples/007-handler-deps](./examples/007-handler-deps) for the full
example.


## Contents

* [Contents](#contents)
* [Components](#components)
* * [Service](#service)
* * * [Independent](#independent)
* * * [Extension](#extension)
* * * [Proxy](#proxy)
* * [Controller](#controller)
* * * [SyncReplier](#syncreplier)
* * * [Replier](#replier)
* * * [Publisher](#publisher)
* * * [Worker](#worker)
* * * [Pair](#pair)
* * [Configuration](#configuration)
* [Further Reading](#further-reading)

---

## Components

## Service
A **service** is a solution for a one problem as an independent
software. An **app** is an interconnection of the services. 

There are three types of services: independent, extension and proxy.

### Independent
Your app should have one independent service
that keeps the core logic of your application.
All app logic is defined as the functions that are bound to the command routes.

Independent services will rarely be shared. So the source code could be private.

### Extension
The extensions are the solutions that could be re-used by multiple projects.

### Proxy
The proxy acts as a switch between a user/service and a user/service. Depending on 
the proxy result the request will be forwarded or returned back to the client.

Forwarding priority is:

1. The proxy handler configuration's `forward` parameter.
2. The message tail, when no configured forward exists for the command.
3. The default outbound, which is the first outbound in the proxy handler config.

The message tail is attached during request deserialization. Configured
forwarding is applied when a whitelisted command in the proxy handler route is
detected, and it overwrites the request outbound before `req.Forward()` is used.

**Limitations**
* proxy service names can not start with `tmp` since it makes the proxy as an ipc protocol for its handlers manager thread which is prohibited.

---
## Handlers
Since the services are the units of distributed system, services
has to talk to each other. And services has to talk with the external world.

Therefore, each service acts as a server. The service mechanism transfers in or out some messages. 
This mechanism is implemented through handlers.

A service may have multiple controllers each on its own socket. 

### SyncReplier
A **SyncReplier** handles a one request at a time. All incoming requests are queued internally, until the current request is not executed.

> The handler always return its result back to the client who called it.

### Replier
A **Replier** handles many requests at a time.
> The handler always return its result back to the client.

### Worker
A **Worker** handles a one request at a time similar to Replier. 

Workers will not respond back to the callee about the status. Its fire-and-forget.

### Publisher
A **Publisher** broadcasts `message.ReplyInterface` to the subscribers. 
To send a message to broadcast, use the publisher's control which has `broadcast` command.

### Pair
A **Pair** connects server to one client. Client and handler both can exchange messages back and forth. To send a message to the client from a handler use the handler's's control.

---

## Configuration
The services keep the topology of proxies and extensions as a json config.
By default its kept as a `noPerfection.json` in the root.
But you can over-write it's path on `service.New(serviceName, **YourPath**)

The hardcoded config of handlers and services set by `SetHandlerConfig` and `SetServiceConfig`
are priority followed by the json config. So, you can stop, edit the ports and start service again.

Note, that each of the service could have it's own configuration, which means it
can have its own extensions and proxies that it can manage by itself.

