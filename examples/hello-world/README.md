# Hello World

This example runs an `Independent` service with the default `main` handler and a separate protocol client.

## Run

Start the service:

```bash
go run ./cmd/service
```

In another terminal, call it:

```bash
go run ./cmd/client
```

Expected output:

```text
hello independent
```

The service registers the `hello` route before `Start()`. Since no handler category is passed to `Route`, it uses the default `main` handler, which is created on `localhost:8000`.

Stop the service with `Ctrl+C`.
