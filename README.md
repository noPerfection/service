# noPerfection/service

Use this go module to turn your go app, package into a noPerfect microservice.

> We omit the **micro** prefix from now on.

Service at its core a collection of [zeromq](https://zeromq.org/) sockets called [handlers](https://github.com/noPerfection/protocol/handler).

# Tutorial 1: Hello World

Install the module

```bash
go get github.com/noPerfection/service
```

Write the main file at `cmd/server/main.go`

```go
package main

import (
	"fmt"

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

func onHello(req service.RequestInterface) service.ReplyInterface {
	name, _ := req.RouteParameters().StringValue("name")

	return req.Ok(map[string]interface{}{"message": "hello " + name})
}
```

Then, launch it `go run ./cmd/service/main.go`, it should be running.

It's time to test by sending our name. Lets create a new app at`cmd/client/main.go`

```go
package main

import (
	"fmt"
	"github.com/noPerfection/service"
)

func main() {
	c, _ := service.Client()
	defer c.Close()

	req := service.RequestMsg("hello", map[string]any{"name": "Jonny Dough"})
	reply, _ := c.Request(req)

	msg, _ := reply.ReplyParameters().StringValue("message")
	fmt.Println(msg)
}
```

if we launch our app on a new terminal tab we will see the greetings:  
`go run ./cmd/client/main.go`.

See [examples/000-hello-world](./examples/000-hello-world) source code.

# Tutorial 2: Why noPerfection

The hello-world example didn't showcase the differentiator about `noPerfection` framework. One of the differentiators is that the service has a built-in handler to manage the service itself remotely or by a code.

By default it's internal accessible by the same process. But we can change its endpoint to expose to the computer or even for a remote control. To do that, we pass the three parameters to our service:

```go
package main

import (
	"github.com/noPerfection/service"
)

func main() {
	// Add the endpoint exposing on port 8001
	controlEndpoint := service.Endpoint("localhost", 8001)
        app, _ := service.New("my-service-name", "noPerfection.json", managerEndpoint)

	app.Route("hello", onHello)

	app.Start()
	defer app.Close()

	app.Wait()
}

func onHello(req service.RequestInterface) service.ReplyInterface {
	return req.Ok(map[string]any{"message": "hello and world"})
}
```

The *"my-service-name"* is a service name to identify it in the topology configuration. The *"noPerfection.json"* is the topology configurations storage path. And last parameter is the handler endpoint.

I'll change the client, to manage our service: I'll use `github.com/noPerfection/os/arg` package to add `--close` flag.
If I pass `--close` flag to the as an argument, then client will talk to its manager asking to close itself. Otherwise it works as in the hello world example.

```go
package main

import (
	"fmt"

	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/service"
	"github.com/noPerfection/service/manager"
)

func main() {
	if arg.FlagExist("close") {
		closeService()
	} else {
		callHello()
	}
}

func callHello() {
	c, _ := service.Client()
	defer c.Close()

	reply, _ := c.Request(service.RequestMsg("hello"))

	message, _ := reply.ReplyParameters().StringValue("message")
	fmt.Println(message)
}

func closeService() {
	c, _ := service.Client("localhost", 8001)
	defer c.Close()

	_, err := c.Request(
		service.RequestMsg(manager.StopService, map[string]any{"service": "my-service-name"}),
	)

        if err != nil {
	        fmt.Println("Service was closed")
        }
}
```

Run the service in one terminal, then run the client in another:

```bash
go run ./cmd/client
go run ./cmd/client --close
```

After `--close`, switch back to the service terminal. The service should be stopped because the manager releases `app.Wait()`.

See [examples/001-close-service](./examples/001-close-service) for the runnable example.

# Tutorial 3: Custom handlers

Handlers are have identifiers in the topology configuration too. They are called category. By default, a service creates a `main` handler. The managers are categorized as `manager`.

Services might have more handlers too but that will be explained in a full documentation.

For now, let's change `main` handler configuration:

- handler type: `SyncReplier`
- endpoint: `localhost:3000`

```go
package main

import (
	"fmt"

	"github.com/noPerfection/service"
)

func main() {
	handlerConf := service.IndependentHandler{
                // identify which handler to set
                Category: "main",
		Type:     service.SyncReplierType, // Was asynchronous Replier, now became SyncReplier
		Endpoint: service.Endpoint("localhost", 3000),
	}

	app, _ := service.New()

	// Define the config for the default "main" handler.
	app.SetHandlerConfig(handlerConf)
        app.Route("hello", onHello)

	app.Start()
	defer app.Close()

	fmt.Println("hello-world service listening on localhost:3000")
	app.Wait()
}

func onHello(req service.RequestInterface) service.ReplyInterface {
	name, _ := req.RouteParameters().StringValue("name")
	return req.Ok(map[string]any{"message": "hello "+name})
}
```

Besides, you can change the handler to act in a different way. As a `service.PublisherType`, as a `service.WorkerType`, as a `service.PairType`. Check out the full documentation how they work. 

SyncReplier means our handler queues the messages and handles them one at a time. It's useful for example if you want to work with the files, or database connections.

If you call `SetHandlerConfig()` after `Start()` it will not have any effect, so call all service configurations before starting it.

Because the handler type changed, the client should connect as a sync replier:

```go
// our cmd/client/main.go
c, _ := service.Client("localhost", 3000, service.SyncReplierType) 
```

---

Just like handlers, you can define service configs from code too:

```go
app.SetServiceConfig(service.Config{
	Type:      service.IndependentType,
	Name:      "hello-world",
	ModuleUrl: "pkg:github.com/noPerfection/service",
	Parameters: service.KeyValue().Set("mode", "tutorial"),
})
```

The `SetHandlerConfig()` and `SetServiceConfig()` are hardcoded configurations. Even if you edit the topology's `.json` file, the hardcoded configurations will be applied every time when you restart the service.

I recommend to use hardcoded configurations minimally, instead edit the topology configuration to have dynamic parameters and then using the manager restart the app.

> The `service.KeyValue()` is a constructor that returns noPerfection/datatype.KeyValue. It's a map[string]any with the additional methods around them. You already saw them in the examples when we looked at the return parameters such as `reply.RouteParameters().StringValue("message")`  is using the `StringValue` of the datatype.KeyValue.

# Framework as a library

Framework usually implies an architectural decisions, code layout in the file systems. Yet, **noPerfection — is a microservices framework as a library**. Topology, and service names, handler categories are providing basic form of architectural constraints.  Routing and handler's socket types are also adds a tiny constraints as well. But its all about the one service's logic.

Apps consists of multiple services, and library provides three types of the services to connect them.
The one that we used so far is called `service.IndependentType` service. Then we have proxies and extensions.

The `noPerfection` app is what I call a reverse scorpio.

noPerfection architecture

Proxies handles the requests and can either forward to the next proxy, or to the independent service or retreive back.

The extensions handles some job on their own threads. 

Another cool thing why to use `noPerfection` is each service manages the topology by itself. 

Note that each service can be a reverse scorpio internally. This nesting can go arbitrarily deep.

## Tutorial 4: Proxy called `default-name-proxy`

Let's modify the hello-world service so it receives a name parameter and greets the caller. But for the client we make it optional. If no name is passed, then service will have a proxy that can set the default name.

 The client accepts an optional `--name=name` argument.

This is a trivial example, but it demonstrates how a proxy can mutate request parameters before they reach the service.

How we do wire up in two steps? First we declare in our topology that there is a proxy, and define its configuration. Then, we tell to our service that `hello` command depends on the proxy using `SetCommandDeps` method:

```go
package main

import (
	"github.com/noPerfection/service"
)

const (
	configPath    = "noPerfection.json"
	serviceName   = "hello-world"
	proxyName     = "default-name-proxy"
)

func main() {
	app, _ := service.New(serviceName, configPath)

	// Tell to our topology about proxy service and where its endpoint to connect too.
	app.Route("hello", onHello)

	app.SetServiceConfig(service.Config{
		Type:      service.ProxyType,
		Name:      proxyName,
		Handlers: []service.Handler{
			service.ProxyHandler{
				Handler: service.Handler{
					Type:     service.SyncReplierType,
					Category: "main",
					Endpoint: message.NewEndpoint("localhost", 8001),
				},
			},
		},
	})
	app.SetCommandDeps(service.DepService{
		Name: "hello",
		Proxies: []string{proxyName},
	})

	app.Start()
	defer app.Stop()

	app.Wait()
}

func onHello(req service.RequestInterface) service.ReplyInterface {
	// Do not verify here, since proxy already did it for us.
	name, _ := req.RouteParameters().StringValue("name")

	return req.Ok(datatype.New().Set("message", "hello "+name))
}
```

Once we have the service, we need to define our proxy. I'll do it in the `cmd/proxy/main.go`.
It handles `service.AnyCmd`, sets `name` to `Medet Ahmetson` when the value is missing or empty, and forwards the request to the service.

```go
package main

import (
	"fmt"

	"github.com/noPerfection/service"
)

func main() {
	app, err := service.NewProxy("default-name-proxy", "noPerfection.json")
	if err != nil {
		panic(err)
	}

	app.Route(base.Any, onDefaultName, "default-name")

	app.Start()
	defer app.Stop()

	fmt.Println("default-name-proxy listening on localhost:8001")
	app.Wait()
}

func onDefaultName(req service.ProxyRequest) service.ProxyReply {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		req.RouteParameters().Set("name", "Medet Ahmetson")
	}

	reply, err := req.Forward()
	if err != nil {
		return req.Fail(err.Error())
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

	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/service"
)

func main() {
	params := service.KeyValue()
	if arg.FlagExist("name") {
		params.Set("name", arg.FlagValue("name"))
	}

	// Connect to the proxy
	c, _ := service.Client("localhost", 8001, service.SyncReplierType)
	defer c.Close()

	reply, _ := c.Request(service.RequestMsg("hello", params))
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

We run the service first, so that service writes the proxy topology.
Since we specified to use the same topology, the proxy after starting will find out its location parameters itself.

The first client call prints `hello Jonny Dough`. The second call omits the
name, so the proxy fills it and the service returns `hello Medet Ahmetson`.

See [examples/004-default-name-proxy](./examples/004-default-name-proxy) for the
runnable example.

## Tutorial 5: Multiple proxies together

Since scorpio tails are long, we can pipe arbitrary number of the proxies. We simply add the second service configuration, and add it into the proxys in the `SetCommandDeps`.

Suppose we want to normalize names to title case:

- `MEDET` becomes `Medet`
- `MEDeT  aHMETSON` becomes `Medet Ahmetson`

We call it `upper-case-names` service. 

So, let's add its configuration into our hello world topology. Add the following piece before calling `SetCommandDeps`:

```go
app.SetServiceConfig(service.Config{
	Type:      service.ProxyType,
	Name:      "upper-case-names",
	Handlers: []service.Handler{
		service.ProxyHandler{
			Handler: service.Handler{
				Type:     service.SyncReplierType,
				Category: "main",
				Endpoint: message.NewEndpoint("localhost", 8003),
			},
		},
	},
})
```

Then, modify the `SetCommandDeps` to have two proxies:

```go
app.SetCommandDeps(service.DepService{
	Name: "hello",
	Proxies: []string{proxyName, "upper-case-names"},
})
```

Proxies are chained in order. The client connects to the first proxy, which forwards to the next, until the last proxy forwards to the service.

The `upper-case-names` proxy implementation is in the examples source — the client and `default-name-proxy` are unchanged from tutorial 4.

Finally run in the terminals:

```bash
go run ./cmd/service
go run ./cmd/proxy
go run ./cmd/proxy2
go run ./cmd/client --name="MEDeT  aHMETSON"
go run ./cmd/client
```

Both client calls print `hello Medet Ahmetson`. The first call formats the
provided name. The second call fills the default name first, then formats it.

See [examples/005-multiple-proxies](./examples/005-multiple-proxies) for the
full example.

## Tutorial 6: Proxy by handler deps

Command deps work well when individual commands need different proxy chains. Let's add another route to our hello world that doesn't need the command proxy.
The `hello` command should use the `default-name` and `upper-case` proxies. But for the `age-verification` command they are redundant.
Without handler deps, the client would need to know two sockets: one is the proxy for default names and one is the direct service endpoint. That forces the client to know about internal topology. Instead, we add an `entrypoint` proxy. The client calls only this entrypoint socket, and internally the entrypoint forwards each command to the right proxy.

Replace the previous `upper-case-names` proxy configuration with an entrypoint:

```go
app.SetServiceConfig(service.Config{
	Type:      service.ProxyType,
	Name:      "entrypoint",
	Handlers: []service.Handler{
		service.ProxyHandler{
			Handler: service.Handler{
				Type:     service.SyncReplierType,
				Category: "main",
				Endpoint: message.NewEndpoint("localhost", 8003),
			},
		},
	},
})

app.Route("age-verification", func(req service.RequestInterface) service.ReplyInterface {
	age, _ := req.RequestParameters().IntValue("age")
	if age >= 18 {
		return req.Ok(map[string]any{"passed": true})
	}
	return req.Ok(map[string]any{"passed": false})
})
```

Then simplify the command dep for `hello` back to a single proxy:

```go
app.SetCommandDeps(service.DepService{
	Name: "hello",
	Proxies: []string{defaultProxyName},
	},
})
```

Finally, set the entrypoint as a handler dep for the service handler:

```go
app.SetHandlerDeps(service.DepService{
	Name: service.DefaultHandlerCategory,
	Proxies: []string{"entrypoint"},
})
```

Rename `proxy2` to `entrypoint` and replace its route handler with the following:

```go
func onForward(req service.ProxyRequest) service.ProxyReply {
	reply, err := req.Forward()
	if err != nil {
		return req.Fail(err.Error())
	}

	return reply
}

// Then in proxy routing:
p.Route(service.AnyCmd, onForward)
```

As for the age verification, let it be a homework, and try to support `--age=<number>` and if its given the client should call the `age-verification` command with the `age` parameter.

When we start the service, the handler dependency will create the correct topology and know to which service it needs to forward the incoming requests. The `hello` command goes through `default-name-proxy`, while `age-verification` goes directly to the service.

Run the service, default-name proxy, entrypoint, and client:

```bash
go run ./cmd/service
go run ./cmd/proxy
go run ./cmd/entrypoint
go run ./cmd/client
go run ./cmd/client --age=21
```

The first client call prints `hello Medet Ahmetson`. The second prints `true`. Both calls use the same entrypoint socket.

See [examples/006-handler-deps](./examples/006-handler-deps) for the full example including the `age-verification` implementation.

The handler deps also provide a facade pattern in distributed systems. We can now reorder the command dep proxies, or later add authentication or name caching without touching the client.

## Tutorial 7: Service that manages other services

One of the differentiators of `noPerfection` is that a service can manage its own proxies and extensions in the same topology. This is arguably the second most powerful feature of `noPerfection`, after being a library.

But so far we were running them in separate terminals. This is because our services are running on TCP ports and the service doesn't know where other services are — it depends on hosting, cloud, and many other nuances. This is intentional.

Every endpoint is defined by a name and a port. When the port is a non-zero number, `noPerfection` uses TCP. But if we set it to `0` and use a name with the `tmp/` prefix, `noPerfection` treats it as a local same-machine endpoint and uses the IPC protocol. For IPC, the service configuration accepts another parameter called `StartCommand`. That's it — to launch a single terminal and let the service manage its own proxies, we simply update the endpoint and set the `StartCommand` parameter. Nothing else changes.

The only thing we need to change are the two `SetServiceConfig` calls:

```go
var defaultnameStartCmd = "go run ./cmd/proxy/main.go"
var defaultnameEndpoint = service.Endpoint("tmp/default_name_proxy", 0)
app.SetServiceConfig(service.Config{
	Type:         service.ProxyType,
	Name:         "default-name-proxy",
	StartCommand: defaultnameStartCmd, // added
	Handlers: []service.Handler{
		service.ProxyHandler{
			Handler: service.Handler{
				Type:     service.SyncReplierType,
				Category: "main",
				Endpoint: defaultnameEndpoint, // updated
			},
		},
	},
})

var entrypointStartCmd = "go run ./cmd/entrypoint/main.go"
var entrypointEndpoint = service.Endpoint("tmp/entrypoint_proxy", 0)
app.SetServiceConfig(service.Config{
	Type:         service.ProxyType,
	Name:         "entrypoint",
	StartCommand: entrypointStartCmd, // added
	Handlers: []service.Handler{
		service.ProxyHandler{
			Handler: service.Handler{
				Type:     service.SyncReplierType,
				Category: "main",
				Endpoint: entrypointEndpoint, // updated
			},
		},
	},
})
```

That's all — now only the service and client need their own terminal. During startup, the service syncs proxy outbounds and then autostarts same-machine dependencies through the topology handler.

Remember from Tutorial 1 that services have managers? They are used by the services themselves to synchronize each other. We had the `close` command earlier, but here, with a topology managed by the service itself, we can talk to its manager and control the entire topology through it. The service will talk to other managers and deliver messages to the right destination.

The full source code and its readme include additional manager commands to start, stop, and check the status of the entrypoint and default-name proxy from the client with the following flags in the full example:

- `./client --help` &ndash; to list all flags &ndash; 
- `./client --services` &ndash; prints the list of services in topology
- `./client --status=<service-name>` &ndash; is service running or not
- `./client --start=<service-name>` &ndash; start service, e.g: *./client --start=entrypoint*
- `./client --stop=<service-name>` &ndash; stop service, e.g: *./client --stop=entrypoint*

Run the service and client:

```bash
go run ./cmd/service
go run ./cmd/client
go run ./cmd/client --age=21
go run ./cmd/client --services
```

See [examples/007-autostart-deps](./examples/007-autostart-deps) for the full example.

## Tutorial 8: Single process

Services are isolated by state. All inter-service communication goes through ZeroMQ sockets, which means it's thread-safe to run multiple services in a single process:

```go
package main

import (
	"github.com/noPerfection/service"
	"github.com/random-org/example"
)

const defaultProxyName = "default-name-proxy"

func main() {
	entrypoint, _ := service.NewProxy("entrypoint")
	entrypoint.Route(service.AnyCmd, example.HandleEntrypoint)

	p, _ := service.NewProxy(defaultProxyName)
	p.Route(service.AnyCmd, example.HandleDefaultName)

	app, _ := service.New()
	app.Route("hello", example.HandleHello)
	app.SetServiceConfig(example.EntrypointConfig)
	app.SetServiceConfig(example.ProxyConfig)

	app.SetCommandDeps(service.DepService{
		Name: "hello",
		Proxies: []string{defaultProxyName},
	})
	app.SetHandlerDeps(service.DepService{
		Name: service.DefaultHandlerCategory,
		Proxies: []string{"entrypoint"},
	})

	// Start app first to set the topology config for other services too
	app.Start()
	entrypoint.Start()
	p.Start()

	// Lock the app until user presses CTR+C
	app.Wait()
}
```

The example above assumes all endpoints use TCP, all started in a single process.

What if you switch one of the endpoints to IPC by setting the port to `0` and prefixing the name with `tmp/`? Well, you start it yourself and the app tries to start it too. It's undefined behavior, so just don't. :)

See [examples/008-single-process](./examples/008-single-process) for the full example. In the example, run the whole demo:

```bash
go run ./cmd/demo
```

Call `hello` through the entrypoint:

```bash
go run ./cmd/client
```

Call `age-verification` through the same entrypoint:

```bash
go run ./cmd/client --age=21
```

List configured services and their running state:

```bash
go run ./cmd/client --services
```

# Multi-stage module progression with configuration change

Why does noPerfection allow multiple services in a single process?

If you set any endpoint's port to `0` and its endpoint's id to anything without the `tmp/` prefix, the service runs on the `inproc` protocol. Inproc is a ZeroMQ transport that allows threads to communicate within a single process. They cannot be reached from any other process. That makes inproc services both fast and secure by design.

The service manager works the same way. As mentioned in tutorial 1, managers are inproc by default. This means the manager is only accessible from within the same process — unless you explicitly expose it on a TCP or IPC endpoint as shown in tutorial 2.

TCP and IPC services are standalone. The calling service doesn't share or know anything about their source code. He needs to know their endpoint. Inproc services are the opposite: they run as threads compiled into the same process, so they require source code.

And here comes the most powerful feature of `noPerfection`, its entire purpose why I made it:

Your app is a collection of modules across directories and files. With noPerfection, each module can evolve on demand:

- **Thread** — the module becomes an inproc service, scoped by its own state, runs as a concurrent thread inside your process. Whether it handles one message at a time, broadcasts, or runs concurrently is up to you, simply choose the socket for handlers.
- **Binary** — as the service grows, you extract it into its own `main()` and binary. In the main application just switch its endpoint to IPC and it runs as a separate process on the same machine. The parent service no longer needs its source, but parent still manages its lifecycle so you dont have to worry about its consistency.
- **Isolated machine** — as it grows further, switch to TCP and deploy it anywhere on the network. The topology handles the rest.
- **Cluster** — as it scales, it gets its own topology with dozens if not hundreds of services, yet nothing changes on the main app logic except the configuration edits.

This progression can also run in reverse. A cluster can be stripped to its core instance, moved back to the same machine, and eventually collapsed back into a thread bundled with the parent app and ultimately into a single package library.

The common denominator across all of these is the package. Package is the shippable piece of code. Although in go, the names are reversed and package is a module. The package depending on the protocol can be used as a library, or used to build your app..

`noPerfection` **handles multi-stage module (a.k.a microservice) progression with just a configuration change**. And its the entire purpose why I created it.

## Tutorial 9: Inprocess services

So let's see the progression using our hello world from tutorial 7: auto start the deps. Instead of IPC I want to make them inproc.

All we need to do in `cmd/service/main.go` is change the configuration:

```go
var defaultnameModuleURL = "pkg:golang/github.com/noPerfection/service/examples/007-autostart-deps#cmd/proxy/main?root=examples/007-autostart-deps"
var defaultnameEndpoint = service.Endpoint("default_name_proxy", 0)
app.SetServiceConfig(service.Config{
	Name:      "default-name-proxy",
	ModuleURL: defaultnameModuleURL,
	// other config
})

var entrypointModuleURL = "pkg:golang/github.com/noPerfection/service/examples/007-autostart-deps#cmd/entrypoint/main?root=examples/007-autostart-deps"
var entrypointEndpoint = service.Endpoint("entrypoint_proxy", 0)
app.SetServiceConfig(service.Config{
	Name:      "entrypoint",
	ModuleURL: entrypointModuleURL,
	// other config
})
```

All we did was two things. First we changed the endpoint by removing the `tmp/` prefix from the ids. Then we added a parameter called `ModuleURL`. Module URLs follow a MushroomURL format which is compatible with the package URL standard.

With inproc protocols, we need to build twice. Let's run our code: `go run ./cmd/service`. As you can see, it didn't run — instead it made code edits. Run it again and it works, listening on the endpoints.

In the first run it created `inproc_topology.go` in the main package. That file defines an extension with all inproc services. It also changed `main.go` to add the extension as belonging to the service manager. The service manager's proxies and extensions run first on startup.

It also updated `go.mod` to include the path to our repository. But if you look at the proxy code, it is defined as `main`. So the first run also extracted the proxy code out of their `main` packages and stored the library versions in `examples/007-autostart-deps/services/entrypoint/service`. Finally it updated the `ModuleURL` in the source to point to the new location.

### Tutorial 10: Inproc Handlers Parameter

Sometimes a proxy handler exposes a TCP or IPC endpoint, but the current process
owns that handler as part of an embedded, single-process topology. In that case
the handler should be treated as in-process for protocol-order validation, even
though its endpoint is remote-capable.

Mark those handler categories in the service parameters:

```json
{
  "type": "Proxy",
  "name": "entrypoint",
  "parameters": {
    "inproc-handlers": ["main"]
  }
}
```

`inproc-handlers` does not change the socket endpoint. It only tells the service
startup validation that the listed handler categories are owned by the current
process. This lets an embedded entrypoint bind TCP or IPC while still forwarding
to hidden inproc command proxies and services.

This parameter is only valid on `Proxy` and `Extension` services. `Independent`
services reject it because they are the top-level service boundary, not an
embedded proxy or extension handler.

Without this parameter, a TCP or IPC proxy handler can not forward into an
inproc handler because inproc endpoints are process-local. With the parameter,
the listed handler is treated as inproc for that validation path.

See [examples/010-inproc-handlers](./examples/010-inproc-handlers) for the full
example note.

### Tutorial 11: security

### Tutorial 12: cross-language

Its cross-language actually, after all services are talking to each other
using zeromq sockets, defining predefined protocol.

So migrating existing code, and starting to make inter thread, and then use rust or C, or go/elixir without making it as an zeromq so its internal.

### Tutorial 13: topology

Topology is pre-built. But you can change it to other popular apps for deployment and management.

#### Cascadefund tutorial 13.1:

once you do it, share it to the people.

## Substrates

Topology configuration is stored as a [Mushroom](https://github.com/ahmetson/mushroom) mycelium. The topology package itself only germinates the **json** colony (`pkg:json`). Other mushroom types are resolved through **substrates** registered by the caller.

The **service** package owns built-in substrates in [`substrates.go`](./substrates.go). When you call `service.New`, `service.NewExt`, or `service.NewProxy`, substrates are passed into `topology.NewHandler` → `config.Load` → `json_substrate.Root`. Topology stays minimal; it does not register substrates on its own.

By default, the service layer supports three mushroom types:

| Type | Module | Role |
|------|--------|------|
| `pkg:golang` | [`github.com/noPerfection/service/package_url`](./package_url/) | Resolves Go module and package links (`module-url`, inproc services, `func=` factories). |
| `pkg:json` | [`github.com/ahmetson/mushroom/substrates/json_substrate`](https://github.com/ahmetson/mushroom/tree/main/substrates/json_substrate) | Loads and mutates topology JSON (`noPerfection.json`). Always used as the root colony. |
| `pkg:os` | [`github.com/noPerfection/os`](https://github.com/noPerfection/os) | Resolves environment links (for example `*pkg:os#env?var=ANTHROPIC_API_KEY&env=.env&envArg=true` in service parameters). |

### Register your own substrate

If you want to add a substrate for another mushroom type, register it before the service loads topology:

```go
import (
	"github.com/noPerfection/service"
)

func init() {
	if err := service.RegisterBuiltinSubstrate(mySubstrate); err != nil {
		panic(err)
	}
}
```

`RegisterBuiltinSubstrate` appends to the built-in list used by every `newTopologyHandler` call. Topology receives the combined list; it never imports your substrate package directly.

Dereference links (`*pkg:…`) inside topology data are fruitized when services are read (for example during `config.Load` validation). Register substrates **before** `service.New` so those links can resolve.

## Contents

- [Contents](#contents)
- [Components](#components)
- - [Service](#service)
- - - [Independent](#independent)
- - - [Extension](#extension)
- - - [Proxy](#proxy)
- - [Controller](#controller)
- - - [SyncReplier](#syncreplier)
- - - [Replier](#replier)
- - - [Publisher](#publisher)
- - - [Worker](#worker)
- - - [Pair](#pair)
- [Substrates](#substrates)
- - [Configuration](#configuration)
- [Further Reading](#further-reading)

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

- proxy service names can not start with `tmp` since it makes the proxy as an ipc protocol for its handlers manager thread which is prohibited.

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