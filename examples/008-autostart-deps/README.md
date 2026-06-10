# Autostart Deps On Same Machine

This example extends Tutorial 7 with same-machine dependency autostart.

The hello-world service keeps its default `localhost:8000` handler and a TCP
manager on `localhost:8001`, while its proxy dependencies use `tmp/` IPC
endpoints and `start-command` values so the service process can launch them
automatically. Only one terminal is needed for the service.

## Run

Start the service:

```bash
go run ./cmd/service
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

Check, start, or stop a dependency service:

```bash
go run ./cmd/client --status=default-name-proxy
go run ./cmd/client --stop=entrypoint
go run ./cmd/client --start=entrypoint
```

Show all client flags:

```bash
go run ./cmd/client --help
```
