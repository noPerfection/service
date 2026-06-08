# Proxy By Handler Deps

This example keeps one public socket for the client while the service still uses
command-specific proxy pipes internally.

The service exposes two commands:

- `hello`
- `age-verification`

The client calls only the `entrypoint` proxy on `localhost:8003`. The entrypoint
forwards every command. For `hello`, handler-dep sync configures a forward to
`default-name-proxy`. For `age-verification`, the entrypoint forwards directly
to the service handler.

## Run

Start the service:

```bash
go run ./cmd/service
```

In another terminal, start the default-name proxy:

```bash
go run ./cmd/proxy
```

In a third terminal, start the entrypoint:

```bash
go run ./cmd/entrypoint
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
