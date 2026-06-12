# Inproc Handlers Parameter

This tutorial documents the `inproc-handlers` service parameter. Use it when a
handler endpoint is TCP or IPC, but that handler is owned by the current process
and should be treated as in-process during protocol-order validation.

The parameter is service-level metadata:

```json
{
  "type": "Proxy",
  "name": "entrypoint",
  "parameters": {
    "inproc-handlers": ["main"]
  },
  "handlers": [
    {
      "type": "SyncReplier",
      "category": "main",
      "endpoint": {
        "id": "localhost",
        "port": 8001
      }
    }
  ]
}
```

The endpoint above still binds TCP on `localhost:8001`. The parameter only says
that the `main` handler belongs to the current process for validation. That
makes it valid for this embedded entrypoint to forward to hidden inproc command
proxies and services.

`inproc-handlers` is only accepted on `Proxy` and `Extension` services.
`Independent` services reject it because they represent the top-level service
boundary.

Without the parameter, this shape is rejected:

```text
entrypoint/main tcp:8001 -> command-proxy/main inproc -> service/main inproc
```

because a remote-capable TCP handler can not directly access process-local
inproc handlers. With `parameters.inproc-handlers: ["main"]`, the validator
treats `entrypoint/main` as inproc for this check:

```text
entrypoint/main (treated as inproc) -> command-proxy/main inproc -> service/main inproc
```

Use this only for handlers that are actually started in the same process as
their inproc outbounds. It does not create a bridge, spawn a service, or change
the socket transport.
