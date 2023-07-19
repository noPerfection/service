# Independent service
> Definition of the independent service is defined on the README here:
[README](README.md)

Create a new go project:

```sh
mkdir my-service
cd my-service
go mod init github.com/account/my-service
```

Get the `service-lib` module and `common-lib` module:

```sh
go get github.com/ahmetson/service-lib
go get github.com/ahmetson/common-lib
go mod tidy
go mod vendor
```

## Internal process
The independent service is composed of the one or more
controllers.

Each of the controllers will have to define the command
names and handlers.

The extensions that are loaded from the configurations
won't be available to the controllers, unless we won't
require by calling:

```go
controller.RequireExtension("database")
```

## Configure

Create the `service.yml` configuration.

```yaml
# Should have at least one service
Services:
  - Type: Independent     # We are defining the independent service
    Name: my-service      # Custom name of the service to classify it.
    Instance: unique-id   # Unique id through this configuration
    Controllers:
      - Name: Name        # Source controller
        Type: Replier     # The type of the controller.
        Instances:
          - Instance: unique-controller
            Port: 8082
      - Name: pub
        Type: Publisher
        Instances:
          - Instance: unique-publisher
            Port: 8083
    Proxies:
      - Name: "" # optional proxies that it depends on
    Pipeline:
      - "proxy->controller" # name of the proxy to bind to the controller name
    Extensions:
      - Name: "database"  # optional extension that it depends on.
                          # the extensions are passed to the handlers.
```

Few notes on the configuration.
The `Services.Type` should be `Independent`, otherwise it won't be valid
to create an independent service. 

If the independent service depends on another independent
service, then other independent service added as an extension.

---

# Independent app

To create an independent service, let's create a `main.go` with the initial service data:

```go
package main

import (
	"github.com/ahmetson/service-lib"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/independent"
	"github.com/ahmetson/service-lib/controller"
	"github.com/account/my-service/handler"
)

func main() {
	logger, appConfig, err := service.New("my-service")
	if err != nil {
		log.Fatal("failed to init service", "error", err)
    }
	
	// setup requirements
	// setup service
}
```

The `service.New("my-service")` is the first thing to call
when you create a service with **Service Lib**.

The function accepts only one argument which is the log prefix.
The function returns the prefixed logger, loaded app configuration
and an error.

Now we are ready to set the independent service starting with the setup.


## Command Name
The first thing to do is to set up a command names.

The messages that independent service accepts are routed to the different
functions. To differentiate the functions, the controllers
use the `command.Name`.

Therefore, let's create a `handler` directory, and `command.go` 
file in the directory.

```go
// ./handler/command.go
package handler

import "github.com/ahmetson/service-lib/communication/command"

const SetCounter command.Name = "set_counter"
const GetCounter command.Name = "get_counter"
```

As you see, we have two commands in this independent service.

## Handler

So, let's create a handler for each of them. 

The handlers are the functions of the type:
`github.com/ahmetson/service-lib/communication/command.HandleFunc`

To do that, let's create
`handler.go` file in `handler` directory.

```go
// ./handler/handler.go
package handler

import (
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/remote"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/common-lib/data_type/key_value"
)

var counter uint64 = 0

var OnSetCounter = func(request message.Request, _ log.Logger, _ remote.Clients) message.Reply {
    newValue, err := request.Parameters.GetUint64("counter")
	if err != nil {
		return message.Fail("no counter in request")
    }
	
	counter = newValue
	
	return message.Reply{
		Status: message.OK, 
		Message: "",
		Parameters: key_value.Empty(),
	}
}

var OnGetCounter = func(_ message.Request, _ log.Logger, _ remote.Clients) message.Reply {
	parameters := key_value.Empty()
	parameters.Set("counter", counter)

	return message.Reply{
		Status: message.OK,
		Message: "",
		Parameters: key_value.Empty(),
	}
}
```

Now we have the two handlers. In any case, the handler always returns `message.Reply`.
If the handler had some error, then message.Reply.Status
will be "FAIL".

## Controller
Now, when we have our command names and their handlers
let's create a controller

```go
controller, err := controller.NewReplier(logger)
// require the extensions for this controller
controller.RequireExtension("database")
// later, independent service will add them to the controller
//
controller.RegisterCommand(handler.GetCounter, handler.OnGetCounter)
controller.RegisterCommand(handler.SetCounter, handler.OnSetCounter)
```

Our controller is ready. But it won't be running.
Since the there is no information about the exposed port of it.

It's added by the Independent service.

## Independent Service

Finally, when our parameters are ready,
we can initialize the independent service.

```go
service, _ := independent.New(appConfig.Services[0])
```

Once we have the service, we need to register the controllers.
This registration will try to get the configuration of the controller
from the `service.yml` by the given controller name.

```go
service.AddController("unique-controller", controller)
```

Finally, we can start the service:

```go
service.Run()
```

The service running will create the controllers,
will create a client to the extensions for each independent controller.