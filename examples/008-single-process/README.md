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

## Why not `tmp/` IPC here?

This example intentionally keeps the proxy endpoints on TCP. If you change the
proxy endpoints to `tmp/...`, the topology treats them as IPC services. IPC
services need a `start-command`, so the main `hello-world` service will try to
autostart those proxies through the topology manager.

That conflicts with the single-process shape of this demo. `cmd/demo` already
imports and starts `default-name-proxy` and `entrypoint` in the current process.
With IPC plus `start-command`, the same app can be started twice: once because
`cmd/demo` calls `Start()`, and once because the topology manager runs the
`start-command`.

The exact behavior depends on startup order and whether the manager probe sees
the in-process proxy as already running. If the proxy is already running, the
topology may skip the extra launch. If not, it may spawn another process, and a
later in-process `Start()` can fail because both copies try to bind the same
IPC socket. For a single-process demo, prefer TCP or in-process endpoints; use
`tmp/` IPC with `start-command` for the multi-process autostart style.

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
