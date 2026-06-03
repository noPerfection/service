# Close Service

This example extends Hello World with a manager endpoint. The service route uses the default `localhost:8000`, and the manager listens on `localhost:8001`.

The client can call the service normally, or send `--close` to ask the manager to stop the service.

## Run

Start the service:

```bash
go run ./cmd/service
```

In another terminal, call the `hello` route:

```bash
go run ./cmd/client
```

Expected output:

```text
hello and world
```

Then ask the manager to close the service:

```bash
go run ./cmd/client --close
```

Expected output:

```text
close signal sent
```

Look back at the service terminal after running `--close`. The service should stop because the manager releases `app.Wait()`.
