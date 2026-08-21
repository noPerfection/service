# noPerfection

**noPerfection** is the **first [zeromq](https://zeromq.org/) based framework** for writing distributed systems.
I wrote it for myself to quickly write backends, deamons without worrying in the technical debts. Super glad to share it for others. Released under **public domain**.

Although on the roadmap is to write the package manager to share `noPerfection` services together as well.
So check out [Ara Foundation](https://ara.foundation/?from=noPerfReadme) for more details.

> For now it has no production code yet except alpha version.

It supports all major zeromq protocol sockets such as `PUB/SUB`, `REQ/REP`, `ROUTER/DEALER`, `PAIR` and `PUSH/PULL`. Additionally introduces new data to organize socket pipes as a modular, scalable yet secure tree.

### Cross-language framework

`noPerfection` also abstracts away the code implementation. Each service gets the universal identity based on [package url.](https://packageulr.org). You can rewrite some services in any programming language. The securitty, network and whole coherent topology will remain unchanged.

Because now, programmers like Rust programming language, they want to rewrite some critical services in rust. With noPerfection its possible to do slowly. One service at a time.

This is the tutorials for a patient reader to start using noPerfection. To save some time, it will omit error checking and other parts. So its not for production.


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

# Understanding noPerfection

The zeromq sockets are divided into handler sockets and client sockets. The handler sockets are bind itself, while client connect to handlers. noPerfection also adds a wrapper around the sockets (for security, circle breakers, etc).
To distinguish from zeromq, the wrappers have their own names for handlers. Client sockets use handler sockets only and detect what zeromq to use internally.


### Socket types

| Handler Socket   | ZMQ Connecting Socket  | ZMQ Binding Socket   |  Description |
| ---------------- | ---------------------- | -------------------- | ------------ |
| `SyncReplier`    | `zmq.REQ`              | `zmq.REP`            | Many clients connect, but one request handling at a time, messages are queued until the handler doesn't execute the previous request. **Returns result** |
| `Replier`        | `zmq.DEALER`           | `zmq.ROUTER`         | Many clients connect, each request is handled concurrently. **Returns result** |
| `Publisher`      | `zmq.SUB`              | `zmq.PUB`            | Many clients subscribe, publisher broadcasts the message. **Fire and forget** |
| `Worker`         | `zmq.PUSH`             | `zmq.PULL`           | Many clients connect, it's similar to `Replier` excet its **fire and forget**. Other variation used by zmq, when a one `zmq.PUSH` is bound is binded and many pullers are not yet supported |
| `Pair`           | `zmq.PAIR`             | `zmq.PAIR`           | One socket connects to another socket. Useful, if you want to secure the inbound, since connected sockets are preventing unauthorized access |

### Protocol: Handlers, Clients and Message

> Source is [github.com/noPerfection/protocol](github.com/noPerfection/protocol)

The message between handler and client are defined in the message format. 


## Service

Service is a group of handlers that act as a single module.
The app is basically collection of the services build in a tree.

There are three types of services: independent, extension and proxy.

* **Independent** should keep core business logic of the application.

* **Extension** are the solutions that could be re-used by multiple projects.

* **Proxy** adds `Forward()` method to the messages.

Forwarding priority is:

1. The proxy handler configuration's `forward` parameter.
2. The message tail, when no configured forward exists for the command.
3. The default outbound, which is the first outbound in the proxy handler config.

The message tail is attached during request deserialization. Configured
forwarding is applied when a whitelisted command in the proxy handler route is
detected, and it overwrites the request outbound before `req.Forward()` is used.

## Configuration

The services keep the topology of proxies and extensions as a json config.
By default its kept as a `noPerfection.json` in the root.
Call `SetTopologyParams(map[string]any{"filepath": "your-path.json"})` before `Start` to use a different file; if you omit it, `Start` uses `DefaultConfigPath` (`noPerfection.json`).

The hardcoded config of handlers, endpoints, and services set by `SetHandlerConfig`, `SetEndpoint`, and `SetServiceConfig`
are priority followed by the json config. So, you can stop, edit the ports and start service again.

Note, that each of the service could have it's own configuration, which means it  
can have its own extensions and proxies that it can manage by itself.

