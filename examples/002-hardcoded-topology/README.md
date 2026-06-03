# Hardcoded Topology

This example extends Hello World with code-defined handler configuration.

Without flags, the service does not provide a handler config. The service creates
the default `main` handler:

- `Replier`
- `localhost:8000`

With `--slow-3000`, the service overwrites the `main` handler config from code:

- `SyncReplier`
- `localhost:3000`

Each request sleeps for `100ms` before replying. The client starts five clients at
the same time, prints request `#1` timing, and prints the total time. This makes
the difference between `Replier` and `SyncReplier` visible.

## Default Replier

Start the service:

```bash
go run ./cmd/service
```

In another terminal, run the client:

```bash
go run ./cmd/client --port=8000
```

## Hardcoded SyncReplier

Start the service with the hardcoded handler config:

```bash
go run ./cmd/service --slow-3000
```

In another terminal, run the client against port `3000`:

```bash
go run ./cmd/client --port=3000
```

The `SyncReplier` handler processes one request at a time, so the total time for
five concurrent clients should be close to five sleeps.
