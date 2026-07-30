# noPerfection

**noPerfection** is a golang framework for writing scalable backend fast without worrying about technical debt. 

> ### 🔥It's a different framework
>
> But it doesn't impose a file structure. NoPerfection comes as a library.
>
> In theory it can work with any applications.

 The goal is to follow the following rule:

- Start your app as a modular monolith for MVP.
- Pick any directory and convert into a microservice living in a thread running parallel. 
- Convert a multi-thread application into a distributed system with multiple processes.
- Convert a process into a cluster of nodes in the cloud.

All, without breaking the business logic itself, because you run them incrementally.

Security is built in. Handshake, liveness and services are managed by the app itself.

Without worrying on data race. noPerfection defines clear dependency tree by defining service types, socket types.

Without worrying long term limitation. Whether you want to create server that replies apps, a deamon that only receives messages or need to broadcast to thousands of connected nodes. They are all supported. And license is public domain too.

> ### 🆓 License is public domain

### Cross-language framework

`noPerfection` also abstracts away the code implementation. Each service gets the universal identity based on [package url.](https://packageulr.org). You can rewrite some services in any programming language. The securitty, network and whole coherent topology will remain unchanged.

Because now, programmers like Rust programming language, they want to rewrite some critical services in rust. With noPerfection its possible to do slowly. One service at a time.

This is the tutorials for a patient reader to start using noPerfection. To save some time, it will omit error checking and other parts. So its not for production.

## Installation

NoPerfection under the hood uses [zeromq](https://zeromq.org/) sockets. Zeromq is a networking library with no message queue, no central broker. With zeromq, the app components communicate to other app components directly via zeromq sockets.

### Install the Zeromq's C library

noPerfection uses the `pebbe/zmq4`, a go wrapper of C library. Check out the [pebbe/zmq4#requirements](https://github.com/pebbe/zmq4#requirements) to download the C library and prepare the operating system itself.

### Install the libsodium

noPerfection uses the Zeromq's CURVE which is based on libsodium cryptography library.
Download libsodium first on your machine as well [jedisct1/libsodium](https://github.com/jedisct1/libsodium).

### Install noPerfection/service core library

```bash
go get github.com/noPerfection/service
```

## Hello World

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
	c, _ := service.NewClient()
	defer c.Close()

	req := service.RequestMsg("hello", map[string]any{"name": "Jonny Dough"})
	reply, _ := c.Request(req)

	msg, _ := reply.ReplyParameters().StringValue("message")
	fmt.Println(msg)
}
```

if we launch our app on a new terminal tab we will see the greetings:  
`go run ./cmd/client/main.go`.

See [github showcase](https://github.com/noPerfection/showcase) for tutorial with readme walkthrough and various examples.

## Substrates

Topology configuration is stored as a [Mushroom](https://github.com/ahmetson/mushroom) mycelium. The topology package itself only germinates the **json** colony (`pkg:json`). Other mushroom types are resolved through **substrates** registered by the caller.

The **service** package owns built-in substrates in `[substrates.go](./substrates.go)`. When you call `SetTopologyParams` or when `Start` creates the default topology handler, substrates are passed into `topology.NewHandler` → `config.Load` → `json_substrate.Root`. Topology stays minimal; it does not register substrates on its own.

By default, the service layer supports three mushroom types:


| Type         | Module                                                                                                                             | Role                                                                                                                                                                                                        |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg:golang` | [github.com/noPerfection/service/package_url](./package_url/)                                                                      | Resolves Go module and package links (`module-url`, inproc services, `func=` factories).                                                                                                                    |
| `pkg:json`   | [github.com/ahmetson/mushroom/substrates/json_substrate](https://github.com/ahmetson/mushroom/tree/main/substrates/json_substrate) | Loads and mutates topology JSON (`noPerfection.json`). Always used as the root colony.                                                                                                                      |
| `pkg:os`     | [github.com/noPerfection/os/substrate](https://github.com/noPerfection/os/tree/main/substrate)                                     | Resolves environment links (for example `*pkg:os#env?var=ANTHROPIC_API_KEY&env=.env&envArg=true` in service parameters). Wired automatically via `ossubstrate.New()` in `[substrates.go](./substrates.go)`. |


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

Dereference links (`*pkg:…`) inside topology data are fruitized when services are read (for example during `config.Load` validation). Register substrates **before** `Start` so those links can resolve.

### Built-in AI extension (`ai`)

`Independent.Start()` registers the built-in `ai` extension under the service manager when it is missing from topology. The factory is `NewAiService()` — the service record is read from topology. Use `SetTopologyParams` before `Start` to point at a custom JSON file.

Service **parameters**:


| Parameter | Default                             | Description                                                                                                                 |
| --------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `api-key` | `*pkg:os/env?var=ANTHROPIC_API_KEY` | Anthropic API key. Stored as a dereference link; mushroom embeds the resolved value when the service is read from topology. |
| `model`   | `claude-haiku-4-5-20251001`         | Anthropic model id (see `mozilla-ai/any-llm-go/providers/anthropic`).                                                       |


`AiService` reads these parameters from topology whenever it needs them (for example on `CheckConnection` or completion calls), so `SetServiceParams` changes take effect without reconstructing the extension.

```go
env.LoadAnyEnv() // or env.LoadAnyEnv(".env")

app.SetServiceParams(datatype.New().
    Set("api-key", "*pkg:os/env?var=ANTHROPIC_API_KEY&env=.env"),
    service.AiServiceName,
)
```

Or construct the extension directly:

```go
ai, err := service.NewAiService()
// ai.SetTopologyParams(map[string]any{"filepath": "noPerfection.json"})
```

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
Call `SetTopologyParams(map[string]any{"filepath": "your-path.json"})` before `Start` to use a different file; if you omit it, `Start` uses `DefaultConfigPath` (`noPerfection.json`).

The hardcoded config of handlers, endpoints, and services set by `SetHandlerConfig`, `SetEndpoint`, and `SetServiceConfig`
are priority followed by the json config. So, you can stop, edit the ports and start service again.

Note, that each of the service could have it's own configuration, which means it  
can have its own extensions and proxies that it can manage by itself.

## Local development

For local testing, building, and running, prefix each command with `GOFLAGS=-modfile=go.local.mod`:

```sh
GOFLAGS=-modfile=go.local.mod go test ./...
GOFLAGS=-modfile=go.local.mod go build ./...
GOFLAGS=-modfile=go.local.mod go run ./cmd/your-app
cd config && GOFLAGS=-modfile=go.local.mod go test ./...
```
