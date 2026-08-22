# noPerfection

**noPerfection** is the **first [zeromq](https://zeromq.org/) based framework** for writing distributed systems.
I wrote it for myself to quickly write backends, deamons and desktop apps without worrying about the technical debts. Super glad to share it for others, so much that its under **public domain** :).


# Understanding **noPerfection** 

This paper introduces the how noPerfection works. From the high level concepts to the lower level concepts.

> **Intended users**
> * use it to start writing apps using noPerfection library.
> * use it to implement noPerfection in other programming language.
> * use it to modify or extend the framework for unexpected cases.

## Mushroom

> defined in [ahmetson/mushroom](https://github.com/ahmetson/mushroom)

Mushroom is opposite of the web, a different form to interconnect the internet data. It's a url format based on the package-url scheme, it aims to make source code internals addressable across any programming languages.

Every service (*explain in the next section*) has its own unique mushroom URL that is equivalent to the purl.

```
pkg:golang/github.com/ahmetson/service
pkg:npm/@lodash/debounce
```

The schema type (golang, npm, pypi) defines the programming language and its package manager.

Following the schema type is the package URL. Its format depends on schema type. Mushroom URL package and schema type are **purl spec matching**. 

> Purl spec already works with the major programming languages and their package managers. So we don't have to invent the URL to share our noPerfection written apps and its modules.

### How noPerfection uses mushroom?
In two ways. Firstly, to identify service's dependency that it interacts via sockets.

Secondly, mushrooms are used to interact with the environment. noPerfection supports one environment mushroom URLs called OS. Use it to interact with the files, network or environment variables within the app. See its docs on [noPerfection/os](https://github.com/noPerfection/os#mushroom-substrate)

### Mushroom's purl extension
The sub package is called a module. The type of module depends on the schema type. Modules in Mushroom URL are defined using `#` separator. 

```
pkg:golang/github.com/ahmetson/service#cmd/zap.go
pkg:npm/@lodash/debounce#src/main.ts
```

> #### Go naming confusion
> In go, package and module namings are reversed. Go modules (go.mod at root) in purl/mushroom are called packages. Go packages in mushroom are called modules.

The modules consists of the code blocks called resources. Static code block called a `var`iable. Dynamically evaluated code block called `func`ion. Combination of both is called `obj`ect. The resource is defined using a `?` separator.

```
pkg:$#src/main.ts?var=VERSION 		-- constant
pkg:$#src/main.ts?obj=User			-- User class
pkg:$#cmd/zap.go?func=NewConnection(id: localhost, port: 3000)
```

### Why mushroom url not web?
Mushroom URLs introduce a `*` operator. *URL with `*` is called a **derefence***, *URL without `*` is called a **link**. When an app reads a file that has derefence, it embeds its value.

```
*pkg:$#src/main.ts?func=print()		-- returns text output

*pkg:$#src/main.ts?obj=User -- returns User's JSON object

```

Check out [Mushroom substrates](#mushroom-substrates) below to manage how your app might understand the URLs.

## Service (a.k.a microservice)
Service is a package that binds to sockets and handles the requests. So it can be either an independent, extension or proxy service. Their combination organizes a socket pipeline as the program architecture.

### Rule of thumb on service type choice

* Use **independent** service for pure business logic.
* Use **proxy** service for everything regarding input/supervising related operations. Inputs like authorization, validation, caching, logging etc. Supervising could be load balancing, routing etc.
* Use **extension** service for everything regarding side-effects/non-socket calls. File operation, database operations, any third party api should have its extension wrapper.

Each **proxy** and **extension** may have its own proxy or extension managed by itself and unknown to dependent service or clients.

Such organization of services as types is called `topology configuration`. The service manages its proxies, and extensions using `topology`.

Check out topology docs and source code:
* [github.com/noPerfection/topology/configuration](github.com/noPerfection/topology/config)
* [github.com/noPerfection/topology](github.com/noPerfection/topology)

### Built-in AI extension (`ai`)

The noPerfection service has a one built in extension `ai`. Its used for converting `main()` to non-main for example.

Service **parameters**:

| Parameter | Default                             | Description                                                                                                                 |
| --------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `api-key` | `*pkg:os/env?var=ANTHROPIC_API_KEY` | Anthropic API key. Stored as a dereference link; mushroom embeds the resolved value when the service is read from topology. |
| `model`   | `claude-haiku-4-5-20251001`         | Anthropic model id (see `mozilla-ai/any-llm-go/providers/anthropic`).                                                       |


`AiService` reads these parameters from topology whenever it needs them (for example on `CheckConnection` or completion calls), so `SetServiceParams` changes take effect without reconstructing the extension.

```go
ai, err := service.NewAiService()
ai.SetServiceParams(datatype.New().
    Set("api-key", "*pkg:os/env?var=ANTHROPIC_API_KEY&env=.env"),
    service.AiServiceName,
)

```

## Sockets

The zeromq sockets are grouped into handler/client pairs. The handler sockets are bind itself, while client connect to remote address. noPerfection also adds a wrapper around the sockets (for security, circle breakers, etc).
To distinguish from zeromq, the wrappers have their own names for handlers. Client sockets use handler sockets only and detect what zeromq to use internally.

Socket types are

| Handler Socket   | ZMQ Connecting Socket  | ZMQ Binding Socket   |  Description |
| ---------------- | ---------------------- | -------------------- | ------------ |
| `SyncReplier`    | `zmq.REQ`              | `zmq.REP`            | Many clients connect, but one request handling at a time, messages are queued until the handler doesn't execute the previous request. **Returns result** |
| `Replier`        | `zmq.DEALER`           | `zmq.ROUTER`         | Many clients connect, each request is handled concurrently. **Returns result** |
| `Publisher`      | `zmq.SUB`              | `zmq.PUB`            | Many clients subscribe, publisher broadcasts the message. **Fire and forget** |
| `Worker`         | `zmq.PUSH`             | `zmq.PULL`           | Many clients connect, it's similar to `Replier` excet its **fire and forget**. Other variation used by zmq, when a one `zmq.PUSH` is bound is binded and many pullers are not yet supported |
| `Pair`           | `zmq.PAIR`             | `zmq.PAIR`           | One socket connects to another socket. Useful, if you want to secure the inbound, since connected sockets are preventing unauthorized access |

### Protocol

> Source is 

Each request from client to handler has a command and parameters. Handler binds each command to the function. It's called **route**: command -> handle func.

Each reply has a status, message and parameters. If status is "fail", then error must have its message. If status is "OK" then optionally may have parameters.

The message format, handler and client implementations + their documentations in the [github.com/noPerfection/protocol](github.com/noPerfection/protocol).

For now for serialization noPerfection uses a json method. The request is transferred as

```json
{
	"command": "greeting",
	"parameters": {
		"name": "Jonny"
	}
}
```

Handler and clients may change it using message packers. The protocol repo has more information about it.

## Configuration

The services keep the topology of proxies and extensions as a json config.
By default its kept as a `noPerfection.json` in the root. If you followed the [hello world](./README.md#hello-world) on Readme, check the `~/hello-world/noPerfection.json`, it should be generated by the app.

Call `SetTopologyParams(map[string]any{"filepath": "your-path.json"})` before `Start` to store parameter differently.

The hardcoded config of handlers, endpoints, and services set by `SetHandlerConfig`, `SetEndpoint`, and `SetServiceConfig`. They are priority, even if you edit configuration, upon restart service will overwrite them to hardcoded ones.

Note, that each of the service could have it's own `noPerfection.json`, as its internal dependency proxies, extensions doesn't have to be known for all in a one place.

## Mushroom Substrates

The mushroom substrate is the URL parser that can do operations on a recognized url.

The built-in substrates are defined in `[./substrates.go](./substrates.go)`. 

### Register your own substrate

```go
import (
	"github.com/noPerfection/service"
)

func init() {
	app, _ := service.New()

	if err := service.RegisterBuiltinSubstrate(mySubstrate); err != nil {
		panic(err)
	}

	app.Start()
}
```


