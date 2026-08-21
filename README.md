# noPerfection

**noPerfection** is the **first [zeromq](https://zeromq.org/) based framework** for writing distributed systems. Written in golang.

I wrote it for myself to quickly write backends, deamons, desktop apps without worrying in the technical debts. Super glad to share it for others. Released under **public domain**.

> For now it has no production code yet except alpha version. For the roadmap and details visit the website [ara.foundation](https://ara.foundation/)

It supports all major zeromq sockets: `PUB/SUB`, `REQ/REP`, `ROUTER/DEALER`, `PAIR` and `PUSH/PULL`. Additionally, it introduces few higher level concepts to organize sockets into a scalable, self-modifiable and secure module tree.

### Roadmap

* ✅ Supports major zeromq sockets, plus TCP, IPC and Inproc protocols.
* ✅ Tutorial and samples [https://github.com/noPerfection/showcase]
* ✅ Security enabled and handled internally using Curve (socket-to-socket) and HMAC (per-route).
* 🠞 API Reference
* 🠞 Docs and Guidelines
* 🠞 Package Manager **CascadeFund**
* 🠞 Protocol spec
* 🠞 Official website

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

2. Create a hello directory for your go source code.

```bash
mkdir hello-world
cd hello-world
```

3. Initialize go to start tracking dependencies and download noPerfection module.

```bash
go mod init example/hello
go get github.com/noPerfection/service
```
4. The hello-world service.
Copy-paste the following source code at `cmd/server/main.go`

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

5. The client that sends a message to the service.
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

6. Test
In the first command prompt start the service.

```bash
go run ./cmd/service
```

On a new terminal run the client.

```bash
go run ./cmd/client/main.go
```

For more tutorials see [showcase & tutorials](https://github.com/noPerfection/showcase/tutorial).
For more details visit the Ara Foundation.

# Installation

### Zeromq's C library & Libsodium

noPerfection uses the `pebbe/zmq4`, a go wrapper around `libzmq` C library. For the CURVE it uses the libsodium library.

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

### Install noPerfection/service core library

It requires go language, if not installed follow the official documentation: [go.dev/doc/install](https://go.dev/doc/install).

Then, in any go project download the noPerfection's module:

```bash
go get github.com/noPerfection/service
```

For more details check out [NO_PERFECTION.md](./NO_PERFECTION.md) for explanation.

## Local development

For local testing, building, and running, use the repo `go.work` file (it wires sibling modules under `noPerfection/`). With it present, plain `go` commands resolve local packages automatically:

```sh
go test ./...
go build ./...
go run ./cmd/your-app
```

Alternatively, prefix commands with `GOFLAGS=-modfile=go.local.mod` if you prefer not to use `go.work`:

```sh
go test -modfile=go.local.mod ./...
go build -modfile=go.local.mod ./...
go run -modfile=go.local.mod ./cmd/your-app
```

---
For more information, visit [ara.foundation](ara.foundation/)
For questions, reach out to me on [Linkedin](linkedin.com/in/ahmetson) or at `milayter @ google's mail com`.

If interesting, please follow the project on social media, until the contribution guides, real world examples release.