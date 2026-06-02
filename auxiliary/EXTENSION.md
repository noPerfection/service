# Extension service
> Definition of the extension service is defined on the README here:
[README](README.md)

Create a new go project:

```sh
mkdir my-extension
cd my-extension
go mod init github.com/account/my-extension
```

Get `github.com/noPerfection/service`, `common-lib` modules:

```sh
go get github.com/noPerfection/service
go get github.com/ahmetson/common-lib
go mod vendor
```

## Configure

> Creating a service.yml is optional.

Create the `service.yml` configuration:

```yaml
# Should have at least one service
Services:
  - Type: Extension       # We are defining the extension service
    Name: my-ext          # Custom name of the service to classify it.
    Instance: unique-id   # Unique id through this config
    Controllers:
      - Name: Name        # Source handler
        Type: Replier     # The type of the handler.
        Instances:
          - Instance: unique-handler
            Port: 8081
    Proxies:
      - Name: "" # optional proxies that it depends on
    Pipeline:
      - "proxy->handler" # name of the proxy to bind to the handler name
    Extensions:
      - Name: "" # optional extension that it depends on.
                 # the extensions are passed to the handlers.
```

Few notes on the configuration.
The `Services.Type` should be `Extension`, otherwise it won't be valid
to create an extension. 
The `Services.Controllers` must have
one element. And the type of the controller should not be

`PUBLISHER`

Extensions are meant to be simple, therefore we have a single
interface.

Additionally, extension may require proxies or extensions.

---

# Extension app

Extensions are the simplest type of services in terms of bootstrapping.
Let's create `main.go`:

```go
package main

import (
	"github.com/noPerfection/service"
	log "github.com/noPerfection/log"
	"github.com/noPerfection/service/extension"
	"github.com/noPerfection/service/configuration"
	"github.com/account/my-extension/handler"
)

func main() {
	logger, _ := log.New("my-extension", false)
	appConfig, _ := configuration.New(logger)

	// setup service
	service, _ := extension.New(appConfig, logger)

	service.AddController(configuration.ReplierType)
	
	// Add the commands and their handlers to the handler
	service.Controller.RequireExtension("github.com/ahmetson/mysql-extension")
	service.Controller.AddRoute(handler.SetCounter)
	service.Controller.AddRoute(handler.GetCounter)
	
	service.Prepare()
	
	service.Run()
}
```

Just like any service, the code starts with the initiation of configuration.
The initialized configuration is assigned to `appConfig`.

Then we do:
* Initialize a new service
* Define the type of the controller.
* Set up the controller: required extensions, handler routes.
* Prepare it by checking configurations.
* Finally, we start the service: `service.Run()`

## Route
We set the `Replier` type of the controller.
Then on `main.go`, we added two routes: `SetCounter` and `GetCounter`.
The routes are defined in the `handler` package.

So let's create `handler.go` in `handler` directory:

```go
// /handler/handler.go
package handler

import (
	"github.com/noPerfection/protocol/client/command"
	"github.com/ahmetson/common-lib/message"
	"github.com/noPerfection/service/remote"
	log "github.com/noPerfection/log"
	"github.com/noPerfection/datatype"
)

// counter is set in the memory for development purpose.
var counter uint64 = 0

var OnSetCounter = func(request message.Request, _ *log.Logger, _ ...*remote.Clients) message.Reply {
	newValue, _ := request.Parameters.GetUint64("counter")
	counter = newValue

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: datatype.New(),
	}
}

var OnGetCounter = func(_ message.Request, _ *log.Logger, _ ...*remote.Clients) message.Reply {
	parameters := datatype.New()
	parameters.Set("counter", counter)

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: datatype.New(),
	}
}

func SetCounter() command.Route {
	return command.NewRoute("set_counter", OnSetCounter)
}
func GetCounter() command.Route {
	return command.NewRoute("get_counter", OnGetCounter)
}
```

The handler type is defined here:
`github.com/noPerfection/service/communication/command.HandleFunc`

Handlers are abstracted as much as possible.
They are intended to focus on the business logic.
Handlers are always returning `message.Reply`.
If the handler fails, then `message.Reply.Status` will be *"OK"*.

During the preparation, the service will fetch the extensions.
Allocate to them random port if it wasn't set on configuration.
