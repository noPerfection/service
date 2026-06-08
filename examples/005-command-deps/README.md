# Command Deps Proxy

This example repeats `004-default-name-proxy`, but the service no longer writes
the proxy handler outbound by hand.

The service declares:

- a proxy service named `default-name-proxy`
- a proxy manager endpoint on `localhost:8002`
- a command dependency for `hello` that points to the proxy

During startup, command dependency synchronization adds the `hello` route and the
service outbound to the proxy handler.

## Run

Start the service:

```bash
go run ./cmd/service
```

In another terminal, start the proxy:

```bash
go run ./cmd/proxy
```

Then call the proxy with a name:

```bash
go run ./cmd/client --name="Jonny Dough"
```

Expected output:

```text
hello Jonny Dough
```

Call the proxy without a name:

```bash
go run ./cmd/client
```

Expected output:

```text
hello Medet Ahmetson
```
