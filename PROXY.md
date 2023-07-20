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
When the *reply handler* is not set, then *proxy controller* returns it to *source*.
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
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/proxy"
)

func main() {
	logger, _ := log.New("my proxy")
	appConfig, _ := configuration.New(logger)

	// setup service
	service := proxy.New(appConfig, logger.Child("proxy"))
	
	// setup a default source
	service.AddDefaultSource(configuration.ReplierType)
	// or
	// service.AddCustomSource(customController)

	// destinations, handlers are part of the controller
	service.Controller.AddDestination(configuration.ReplierType)
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

The `appConfig` is always needed for any service created with **Service Lib**.
AppConfig should get the logger at the root level. Because it
will get the app name from the logger.

It loads the environment variables, and optionally `service.yml`.

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


