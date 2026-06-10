# Single Process Demo

So far the examples started services and proxies from different commands or
let the main service autostart proxy processes. This tutorial keeps the same
hello, age-verification, and two-proxy topology, but runs everything from one
Go package.

The demo uses TCP endpoints for every service:

- `hello-world` listens on `localhost:8000`
- `default-name-proxy` listens on `localhost:8002`
- `entrypoint` listens on `localhost:8004`

`cmd/demo` imports the reusable `hello`, `proxy`, and `entrypoint` packages,
starts all three services in the same process, and then waits for all of them.

## Run

Start the whole topology:

```bash
go run ./cmd/demo
```

Call `hello` through the entrypoint:

```bash
go run ./cmd/client
```

Expected output:

```text
hello Medet Ahmetson
```

Call `age-verification` through the same entrypoint:

```bash
go run ./cmd/client --age=21
```

Expected output:

```text
true
```

List configured services and running state through the service manager:

```bash
go run ./cmd/client --services
```

Check a service status:

```bash
go run ./cmd/client --status=default-name-proxy
```

Show all client flags:

```bash
go run ./cmd/client --help
```
