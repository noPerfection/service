# Proxy service
> Definition of the proxy service is defined on the README here:
[README](README.md)

Create a new go project:

```sh
mkdir my-proxy
cd my-proxy
go mod init github.com/account/my-proxy
```

Get the `service-lib` module:

```sh
go get github.com/ahmetson/service-lib
go mod tidy
go mod vendor
```

## Internal process

The proxy contains the three parts. *source*, *destination*,
*request handler*, *reply handler*. 
The proxy service has the internal controller that binds all four: *proxy controller*.

> The *proxy controller* is set in `inprox://proxy_router`. 

The *source* is a controller. The *handler*
is the function. And the *destination* is the client. A client
that connects to a controller of another service.

The source controller receives the messages from the external world.
It then passes it to *proxy controller*. The proxy controller
executes the *handler* function. Handler function returns two parameters to
the *proxy controller*: message and error.
If there is an error, then *proxy controller* returns error back to the *source
controller*.
If there is no error, then *proxy controller* redirects the message to the
*destination*.

The destination client gets the reply from the destination service.
If the destination gets the reply, then it passes it to *proxy controller*.
The proxy controller checks for the optional *reply handler*.
If there is a reply handler, then call reply handler. If reply handler
failed, then return to the source controller error. Otherwise,
return the message returned from the reply handler.
---

## Configure

Now we understand the internal process of the proxy service. So, let's
start to write it. The first thing is the `service.yml` configuration.

```yaml
# Should have at least one service
Services:
  - Type: Proxy           # We are defining the proxy service
    Name: my-proxy        # Custom name of the service to classify it.
    Instance: unique-id   # Unique id through this configuration
    Controllers:
      - Name: source      # Source controller
        Type: Replier     # The type of the controller.
        Instances:
          - Instance: unique-source
            Port: 8080
      - Name: destination # Destination controller parameters to create a client
        Type: Replier
        Instances:
          - Instance: unique-destination
            Port: 8080
    Proxies:
      - Name: "" # optional proxies that it depends on
    Pipeline:
      - "proxy->controller" # name of the proxy to bind to the controller name
    Extensions:
      - Name: "" # optional extension that it depends on.
                 # the extensions are passed to source, request handler to reply handler.
```

Few notes on the configuration.
The `Services.Type` should be `Proxy`, otherwise it won't be valid
to create a proxy. The `Services.Controllers` must have
two elements. The first element's name should be *'source'*.
The second element's name should be *'destination'*. These controller
names are preserved by the *proxy controller*.

The controllers may be custom. The custom controllers are
based on the base types that **Service lib** provides.
In the controller is of the custom type, then use the base controller type.

Before preparing the proxy service itself, let's go the over the setups.
These setups should be passed to the proxy controller.

---

# Proxy app

To create a proxy, let's create a `main.go` with the initial service data:

```go
package main

import (
	"github.com/ahmetson/service-lib"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/proxy"
)

func main() {
	logger, appConfig, err := service.New("my-proxy")
	if err != nil {
		log.Fatal("failed to init service", "error", err)
    }
	
	// setup requirements
	// setup service
}
```

The `service.New("my-proxy")` is the first thing to call
when you create a service with **Service Lib**.

The function accepts only one argument which is the log prefix.
The function returns the prefixed logger, loaded app configuration
and an error.

Now we are ready to set the proxy service starting with the setup.

## Source
The first thing to do is to set up a source controller.

After the creation, the source controller should be added
to the proxy service. When a proxy service runs,
it will add the *proxy controller* as the extension to the
*source controller*.

If you are not creating the custom controller,
then you can Source section.

### Custom Source
The custom source controllers should implement the interface:
`github.com/ahmetson/service-lib/controller.Interface`.

## Request Handler
The next thing is to define the request handler.
The request handler is the function of the type:

`github.com/ahmetson/service-lib/proxy.RequestHandler`

## Reply Handler

> This is optional

If you want to do some operation or convert the data
then you can optionally define the reply handler.

The reply handler is the function of the type:

`github.com/ahmetson/service-lib/proxy.ReplyHandler`

## Proxy Service

Finally, when our parameters are ready,
we can initialize the proxy service.

```go
service, _ := proxy.New(appConfig.Services[0], logger)
```

We need to set up the source controller to the proxy service.


```go
// if the source controller, then add as this:
service.AddSourceController(proxy.SourceName, web)

// otherwise, if we want to use the built-in controller:
service.NewSourceController(configuration.ReplierType)
```

We need to set the request handler

```go
service.SetRequestHandler(requestHandler)
```

Optionally, we need to set up the reply handler

```go
service.SetReplyHandler(replyHandler)
```

Finally, we can start the service:

```go
service.Run()
```
