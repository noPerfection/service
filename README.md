# noPerfection

**noPerfection** is the **first [zeromq](https://zeromq.org/) based framework** for writing distributed systems. Written in golang.

I wrote it for myself to quickly write backends, deamons, desktop apps without worrying in the technical debts. Super glad to share it for others. Released under **public domain**.

> For now it has no production code yet except alpha version. For the roadmap and details visit the website [ara.foundation](https://ara.foundation/)

It supports all major zeromq sockets: `PUB/SUB`, `REQ/REP`, `ROUTER/DEALER`, `PAIR` and `PUSH/PULL`. Additionally, it introduces few higher level concepts to organize sockets into a scalable, self-modifiable and secure module tree.

### Roadmap

- ✅ Supports major zeromq sockets, plus TCP, IPC and Inproc protocols.
- ✅ Tutorial and samples [https://github.com/noPerfection/showcase/tutorial](https://github.com/noPerfection/showcase/tutorial)
- ✅ Security enabled and handled internally using [Elliptic Curves](https://en.wikipedia.org/wiki/Curve25519) (socket-to-socket) and [HMAC](https://en.wikipedia.org/wiki/HMAC) (per-route).
- 🠞 **2 production ready use cases**
- 🠞 API Reference
- 🠞 Documentation and guideline
- 🠞 Package Manager **CascadeFund**
- 🠞 Protocol spec, available draft is at [NO_PERFECTION.md](./NO_PERFECTION.md).
- 🠞 Official website


## Hello World

First follow the [installation](./README.md#installation) instructions, then create a new golang project.

1. Open a command prompt and cd to your home directory.

On Linux or Mac:

```bash
cd
```

On Windows:

```bash
cd %HOMEPATH%
```

1. Create a hello directory for your go source code.

```bash
mkdir hello-world
cd hello-world
```

1. Initialize go to start tracking dependencies and download noPerfection module.

```bash
go mod init example/hello
go get github.com/noPerfection/service
```

1. The hello-world service.

Copy-paste the following source code at `cmd/service/main.go`

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

1. The client that sends a message to the service.

Copy-paste the following source code at`cmd/client/main.go`

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

1. Test

In the first command prompt start the service.

```bash
go run ./cmd/service
```

On a new terminal run the client.

```bash
go run ./cmd/client/main.go
```



# noPerfection features

1. **It's the first framework based on [zeromq](https://zeromq.org/)**.
2. **It's a framework as a library**: no predefined file structure, directory layout up to you.
3. **Structurized microservice mesh**: instead it organizes runtime structure as socket pipelines with service concept.
4. **Security built-in and hardcoded**: for socket pipeline noPerfection uses two factor security: CURVE keys and HMAC. 
5. **scale-on-demand**: start noPerfection app as an MVP. Convert into a concurrent thread any sub-module by adding a socket. To turn a thread into a separate OS process or cloud cluster, simply change the service's configuration.
6. **Manages dependency itself**: the main service starts and relaunches dependency tree itself.
7. **language, environment agnostic**: rewrite any module in [zeromq bindings list](http://wiki.zeromq.org/bindings:_start) available languages. microservices also doesn't know where the service is? same process, same computer or remote, since it abstracts environment and manages it itself. 
8. **modular**: message format, language support, socket or service types, and environments interaction are replaceable.

---

> ### **The long term goal of noPerfection is to make self-modifiable apps which live on the internet and serves people's request from your computer**. 

The tutorial walkthrough covering noPerfection features are available on [noPerfection tutorial](https://github.com/noPerfection/showcase/tutorial). For more details visit [Ara Foundation](https://ara.foundation/).

# Installation

### Zeromq's C library & Libsodium

noPerfection uses the `pebbe/zmq4`, a go wrapper around `libzmq` C library. For the CURVE it uses the `libsodium`.

On **Windows**, better to use [vcpkg](https://vcpkg.io/en/):

```bash
vcpkg install zeromq[sodium]
```

For more details [vcpkg.link/zeromq](https://vcpkg.link/ports/zeromq).

---

On **Linux** or **OSX**, the zeromq has the official installation steps on [zeromq.org/download/](https://zeromq.org/download/).

---



#### Libsodium MacOS

```bash
brew install libsodium
```



#### Libsodium Linux Fedora

```bash
sudo dnf install libsodium-devel gcc
```



#### Libsodium Linux Ubuntu/Debian/Mint

```bash
sudo apt update
sudo apt install libsodium-dev build-essential
```



#### Libsodium Linux Arch

```bash
sudo pacman -Syu libsodium base-devel
```



#### Libsodium Linux SUSE

```bash
sudo zypper in libsodium-devel devel_basis
```

---

