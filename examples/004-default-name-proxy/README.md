# Default Name Proxy

This example keeps the `hello-world` service strict: the `hello` route requires a
`name` parameter and fails when the client does not send one.

The proxy owns the extra input behavior. Clients connect to the proxy on
`localhost:8001`, and the proxy fills `name` with `Medet Ahmetson` when the
parameter is missing or empty. Then it forwards the request to the service's
`main` handler on `localhost:8000`.

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

---
Testing the reply, and failure without forwarding.
If you type a `loser` as a name, it should return success:

```bash
go run ./cmd/client --name="loser"
```
Expected output:

```text
Who is loser?
```

If you type any name that contains `shit` it should return failure:

```bash
go run ./cmd/client --name="bullshit"
```

Expected output:

```bash
panic: I'll tell your mom
```