# Multiple Proxies Together

This example repeats `005-command-deps`, but the `hello` command goes through two
proxies:

- `default-name-proxy` fills the missing name.
- `upper-case-names` formats the name after a name exists.

The command dependency order is the proxy chain order. The first proxy in the
list is the one the client calls.

## Run

Start the service:

```bash
go run ./cmd/service
```

In another terminal, start the first proxy:

```bash
go run ./cmd/proxy
```

In a third terminal, start the second proxy:

```bash
go run ./cmd/proxy2
```

Then call the first proxy with an unformatted name:

```bash
go run ./cmd/client --name="MEDeT  aHMETSON"
```

Expected output:

```text
hello Medet Ahmetson
```

Call the first proxy without a name:

```bash
go run ./cmd/client
```

Expected output:

```text
hello Medet Ahmetson
```
