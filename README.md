# Service Lib

> SDS connects all developers to help each other.
> 
> SDS connects all APIs in the world.
> 
> Need to find a customer for your service? Use this SDS.
> 
> ### How it works?
> 
> You as developer -> your app -> sds -> third party
> 
>                                        -----------
>                                        database (sql, cache)
>                                        cdn (cloudflare, aws)
>                                        auth (google, custom)
>                                        payment (crypto, fiat)
> SDS creates a single interface to access all API.
> 
> 
> other developer -> other app -> sds -> third party
>  
>                                        -----------
>                                        database (sql, cache)
>                                        your app
> 
> SDS makes your app as a service for other developers.

**Service Lib** module is used to
create various inter-connectible backend services.

There are three types of services:
* Independent
* Proxy
* Extension

The independent services are stand-alone service
that aims to solve one specific challenge.

> Example of independent service:
> * Reading smartcontract data
> * Sending data to the smartcontract

The extension services are handling the task that could
be shared by many services. 

> Example of extension services:
> * Database connection (SQL)
> * Database connection (Filesystem)
> * Store private keys in Vault
> 
> *All extensions could be used by many services for their own needs*

Finally, the **proxy services** are set between external
user and destination service. If there is a proxy,
then the destination service will block its own controller.
Instead, the destination service will receive data only from
the proxy service.
Proxy can be nested to each other by organizing a pipeline.

> Example of proxy services:
> * Web (enable Http portal)
> * WebSocket (enable the websocket protocol)
> * Auth (authenticate the message)
> * BSON (rather than json get the data in BSON format)
> * Validate

## Configuration
Any apps created by this module is loading environment
variables by default.

As well as it requires the *configuration* in yaml format.

You can set the Yaml file name as well as it's path
using the following environment variables:
```bash
SERVICE_CONFIG_NAME=service
SERVICE_CONFIG_PATH=.
```

By default, the service will look for `service.yml` in the `.` directory.

The configuration format is this:
```yaml
Services:
  - Type: Independent # or Proxy | Extension
    Name: 
    Instance: 
    Controllers:
      - Type: Replier # or Puller | Publisher | Router
        Name: "myApi"
        Instances:
          - Port: 2302
            Instance: 
    Proxies:
      - Name: "auth"
        Port: 8000

    Pipelines:
      - "auth->myApi"

    Extensions:
      - Name: "database"
        Port: 8002
```

At root, it has `Services` with at least one Service defined.
The service has the following parameters:

* Type which defines what kind of service it is. It could be `Independent`, `Proxy` or `Extension`.
* Name of the service. If you define multiple services, then their Type and Name should match.
* Instance is the unique identifier of this service. If you have multiple services, then it should have different instance.
* Controllers lists what kind of command handlers it has.
* Proxies lists what kind of proxies it has.
* Pipelines should have one or more proxy pipeline. The last name should name of the controller instance.
* Extensions lists the extensions that this service depends on. All these extensions are passed to the controllers.

The **controllers** are the command handlers. All incoming requests
from the users (whether it's through proxy or not) are handled by the controllers.
The parameters of the controllers:
* Type which defines what kind of controller it is. It could be Replier, Puller, Publisher or Router.
* Name of the controller to classify it.
* Instances describes the unique controllers of this type.

The controller instances have the following parameters:
* Instance is the unique id of the controller within all service
* Port where the controller exposes itself

Proxy has the following parameters:
* Name of the proxy
* Port where the proxy set too.

Extension has the following parameters:
* Name of the extension
* Port where the extension set too.

## Proxy service
The proxy service should have at least two controllers:
*source* and *destination*.

The *source* controller is created by the developer or
the proxy service. The users are connecting to the 
*source* controller.

Once the data is received from the *source*, the proxy service
sends it to the next controller: *destination*.
The destination is not the controller in the proxy, rather
it's the controller on another service.

The proxy itself creates a router controller set in: 
`inprox://proxy_router`. The proxy router accepts only
one single command handler. Any incoming message is passed
to the handler.

The *source* controller has a client connected to the router.
While the router itself has a client connected to *destination*.