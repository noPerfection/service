# Extension service
> Definition of the extension service is defined on the README here:
[README](README.md)

Create a new go project:

```sh
mkdir my-extension
cd my-extension
go mod init github.com/account/my-extension
```

Get the `service-lib` module and `common-lib` module:

```sh
go get github.com/ahmetson/service-lib
go get github.com/ahmetson/common-lib
go mod tidy
go mod vendor
```

## Configure

Unlike proxy, the extensions are easy to understand.
Therefore, we skip the internal process explanation and
straightforward will work with the cofiguration.
Create the `service.yml` configuration.

```yaml
# Should have at least one service
Services:
  - Type: Extension       # We are defining the extension service
    Name: my-ext          # Custom name of the service to classify it.
    Instance: unique-id   # Unique id through this configuration
    Controllers:
      - Name: Name        # Source controller
        Type: Replier     # The type of the controller.
        Instances:
          - Instance: unique-controller
            Port: 8081
    Proxies:
      - Name: "" # optional proxies that it depends on
    Pipeline:
      - "proxy->controller" # name of the proxy to bind to the controller name
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

To create an extension, let's create a `main.go` with the initial service data:

```go
package main

import (
	"github.com/ahmetson/service-lib"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/extension"
	"github.com/account/my-extension/handler"
)

func main() {
	logger, appConfig, err := service.New("my-extension")
	if err != nil {
		log.Fatal("failed to init service", "error", err)
    }
	
	// setup requirements
	// setup service
}
```

The `service.New("my-extension")` is the first thing to call
when you create a service with **Service Lib**.

The function accepts only one argument which is the log prefix.
The function returns the prefixed logger, loaded app configuration
and an error.

Now we are ready to set the extension service starting with the setup.

## Command Name
The first thing to do is to set up a command names.

The messages that extension accepts are routed to the different
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

As you see, we have two commands in this extension.

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

## Extension Service

Finally, when our parameters are ready,
we can initialize the extension service.

```go
service, _ := extension.New(appConfig.Services[0], logger)
```

Once we have the service, we need to add register our
commands in it's controller:

```go
controller := service.GetFirstController()
controller.RegisterCommand(handler.GetCounter, handler.OnGetCounter)
controller.RegisterCommand(handler.SetCounter, handler.OnSetCounter)
```

Finally, we can start the service:

```go
service.Run()
```
