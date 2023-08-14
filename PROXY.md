# Proxy service
> Definition of the proxy service is defined on the README here:
[README](README.md)

Create a new go project:

```sh
mkdir my-proxy
cd my-proxy
go mod init github.com/account/my-proxy
```

Get `service-lib` module:

```sh
go get github.com/ahmetson/service-lib
go mod vendor
```

## Internal process

The proxy service is composed of: 
* *source* &ndash; a controller
* *destination* &ndash; a client. It's connected to the remote service controller,
* *request handler* &ndash; a function, 
* *reply handler* &ndash; a function,
* *proxy controller* &ndash; an internal controller binding all above.

> The *proxy controller* is set on `inprox://proxy_router`. 

The *source* controller receives the messages. 
The incoming requests are then passed to *proxy controller*. 
The *proxy controller* executes the *request handler*.  
If handling passes the execution, then the message is sent to the *destination*.

The *reply handler* is an optional function.
When it's set, the replies from *destination* is executed with *reply handler*.
The result of the execution is returned to the *source*.
When the *reply handler* is not set, then *proxy controller* sends directly to *source*.
---

## Configure

Now we understand the internal process of the proxy service. So, let's
start to write it. The first thing is the `service.yml` configuration.

```yaml
# Should have at least one service
Services:
  - Type: Proxy           # We are defining the proxy service
    Url: url        # Custom name of the service to classify it.
    Instance: unique-id   # Unique id through this config
    Controllers:
      - Name: source      # Source handler
        Type: Replier     # The type of the handler.
        Instances:
          - Instance: unique-source
            Port: 8080
      - Name: destination # Destination handler parameters to create a client
        Type: Replier
        Instances:
          - Instance: unique-destination
            Port: 8080
    Proxies:
      - Url: "" # optional proxies that it depends on
    Pipeline:
      - "proxy->handler" # name of the proxy to bind to the handler name
    Extensions:
      - Url: "" # optional extension that it depends on.
                 # the extensions are passed to source, request handler to reply handler.
```

Few notes on the configuration.
The `Services.Type` should be `Proxy`. 
The `Services.Controllers` must have two elements. 
The first controller should be named *'source'*.
The second controller should be named *'destination'*. 
The names are preserved by the *proxy controller*.

The source controllers may be custom or built in ones. 
The custom controllers are based on the builtin types.

---

# Proxy app

Let's create `main.go` with the minimal *proxy*:

```go
package main

import (
	"github.com/ahmetson/service-lib"
	log "github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/proxy"
)

func main() {
	logger, _ := log.New("my-proxy", false)
	appConfig, _ := configuration.New(logger)

	// setup service
	service := proxy.New(appConfig, logger.Child("proxy"))
	
	// setup a default source
	service.SetDefaultSource(configuration.ReplierType)
	// or
	// service.SetCustomSource(customController)

	// destinations, handlers are part of the handler
	service.Controller.RequireDestination(configuration.ReplierType)
	service.Controller.SetRequestHandler()
	service.Controller.SetReplyHandler()
	
	// validate before running it
	service.Prepare()
	
	service.Run()
}
```

> TODO
> 
> Before adding source, add the extensions
> That means, we need to change the request handler, reply handler.
> The handlers in the controller should receive extensions.
> ```go
> // List the dependency
> //service.RequireExtension("")
> ```

All services start with the configuration setup.
On `main.go` the configuration is set to `appConfig`.

Then we do:
* Initialize a new proxy
* Set up: source, destination and handlers.
* Prepare it by checking configurations.
* Finally, we start the service: `service.Run()`

## Source
The *proxy controller* will add itself to *source* controller automatically.
The handlers in the *proxy controller* should send the messages to *proxy controller*.

### Custom Source
The custom source controllers should implement the interface:
`github.com/ahmetson/service-lib/controller.Interface`.

> Check the example on https://github.com/ahmetson/web-proxy

## Request Handler
The request handler is the function of the type:

`github.com/ahmetson/service-lib/proxy.RequestHandler`

## Reply Handler

> This is optional

The reply handler is the function of the type:

`github.com/ahmetson/service-lib/proxy.ReplyHandler`
