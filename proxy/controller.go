package proxy

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/remote"
	zmq "github.com/pebbe/zmq4"

	"github.com/ahmetson/service-lib/log"
)

// RequestHandler handles all raw requests from source.
//
// Returns the raw message to send to the destination
type RequestHandler = func([]string, log.Logger) ([]string, error)

// ReplyHandler handles all raw requests from destination.
// The first argument is the raw message.Reply received from the destination.
// The second argument is the request messages received from the client (without delimiter and id)
//
// Returns the raw message to send to the source.
type ReplyHandler = func([]string, []string, log.Logger) []string

// ControllerName is the name of the proxy router that connects source and destination
const ControllerName = "proxy_controller"

// Url defines the proxy controller path
const Url = "inproc://" + ControllerName

// Destination is the client connected to the controller of the remote service.
// The Dealer is the Request from Router to the
// Reply Controller.
//
// The socket.Type must be zmq.DEALER
type Destination struct {
	// Could be Remote or Inproc
	Configuration *configuration.Controller
	// The client socket
	socket *zmq.Socket
}

// Controller is the internal process connecting source and destination.
type Controller struct {
	destination *Destination
	// type of the required destination
	requiredDestination configuration.Type
	logger              log.Logger
	requestHandler      RequestHandler
	replyHandler        ReplyHandler
	requestMessages     *key_value.List
}

// newController Returns the initiated Router with the service parameters that connects source and destination.
// along within the route, it will execute request handler and reply handler.
func newController(logger log.Logger) *Controller {
	return &Controller{
		logger:          logger,
		destination:     nil,
		requestHandler:  nil,
		replyHandler:    nil,
		requestMessages: key_value.NewList(),
	}
}

// SetRequestHandler sets the request handler.
// If the request handler succeeds then the request handler will have the final message
// in the "destination"
func (controller *Controller) SetRequestHandler(handler RequestHandler) {
	controller.requestHandler = handler
}

// SetReplyHandler sets the reply handler.
// The reply handler's error will be printed, but it doesn't mean that client will receive it.
func (controller *Controller) SetReplyHandler(handler ReplyHandler) {
	controller.replyHandler = handler
}

func (controller *Controller) RequireDestination(controllerType configuration.Type) {
	controller.requiredDestination = controllerType
}

// RegisterDestination Adds a new client that is connected to the Reply Controller.
// Verification of the service limit or service protocol type
// is handled on outside. As a result, it doesn't return
// any error.
// SDS Core can have unique command handlers.
func (controller *Controller) RegisterDestination(destinationConfig *configuration.Controller) {
	controller.logger.Info("Adding client sockets that router will redirect", "destinationConfig", *destinationConfig)

	controller.destination = &Destination{Configuration: destinationConfig, socket: nil}
}

// Internal function that assigns the socket
// to the Clients.
//
// It's handled in this not in the socket.
// Because called from the router goroutine (go router.Run())
//
// If the Router creating thread calls
// then as thread-unsafely, will lead to the unexpected
// behaviours.
func (controller *Controller) setDestinationSocket() error {
	socket, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = socket.Connect(remote.ClientUrl(controller.destination.Configuration.Instances[0].Name, controller.destination.Configuration.Instances[0].Port))
	if err != nil {
		return fmt.Errorf("setup of dealer socket: %w", err)
	}

	controller.destination.socket = socket

	return nil
}

// Run the router (asynchronous zmq.REP) along with dealers (asynchronous zmq.REQ).
// The dealers are connected to the zmq.REP controllers.
//
// The router will redirect the message to the zmq.REP controllers using dealers.
//
// Note!!! If the request to this router comes from zmq.REQ client, then client
// should set the identity using zmq.SetIdentity().
//
// Format of the incoming message:
//
//		0 - bytes request id
//		1 - ""; empty delimiter
//		2 - string (app/parameter.ServiceType) service name as a tag of dealer.
//	     to identify which dealer to use
//		3 - app/remote/message.Request the message that is redirected to the zmq.REP controller
//
// The request id is used to identify the client. Once the dealer gets the reply from zmq.REP controller
// the router will return it back to the client by request id.
//
// Example:
//
//	// route the msg[3] to the SDS Storage
//	msg := [0: "uid-123", 1: "", 2: "storage", 3: "{`command`: `smartcontract_get`, `parameters`: {}}"]
func (controller *Controller) Run() {
	if controller.destination == nil {
		controller.logger.Fatal("no destinations registered in the proxy", "hint", "call router.RegisterDestination()")
	}

	if controller.requestHandler == nil {
		controller.logger.Fatal("request handler wasn't set")
	}

	controller.logger.Info("setup the dealer sockets")
	//  Initialize poll set
	poller := zmq.NewPoller()

	// let's set the socket
	err := controller.setDestinationSocket()
	if err != nil {
		controller.logger.Fatal("setDestinationSocket", "destination instance", controller.destination.Configuration.Instances[0].Instance)
	}
	poller.Add(controller.destination.socket, zmq.POLLIN)
	controller.logger.Info("dealers set up successfully")
	controller.logger.Info("setup router", "url", Url)

	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer func() {
		err := frontend.Close()
		if err != nil {
			controller.logger.Fatal("failed to close the socket", "error", err)
		}
	}()
	err = frontend.Bind(Url)
	if err != nil {
		controller.logger.Fatal("zmq new router", "error", err)
	}
	hwm, _ := frontend.GetRcvhwm()
	controller.logger.Warn("high watermark from router", hwm)

	poller.Add(frontend, zmq.POLLIN)

	controller.logger.Info("The proxy controller waits for client requestMessages")

	if controller.replyHandler != nil {
		controller.logger.Warn("the reply handler was given, we will track all messages",
			"todo 1", "clean the messages after timeout")
	}

	//  Switch messages between sockets
	for {
		// The '-1' argument indicates waiting for the
		// infinite amount of time.
		sockets, err := poller.Poll(-1)
		if err != nil {
			controller.logger.Fatal("poller", "error", err)
		}
		for _, socket := range sockets {
			zmqSocket := socket.Socket
			// redirect to the dealer
			if zmqSocket == frontend {
				messages, err := zmqSocket.RecvMessage(0)
				if err != nil {
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						controller.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				// Let's bypass the string
				destinationMessages, err := controller.requestHandler(messages[2:], controller.logger)
				if err != nil {
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						controller.logger.Fatal("replyErrorMessage", "error", err)
					}
					continue
				}
				if len(destinationMessages) > 2 &&
					destinationMessages[0] == messages[0] &&
					destinationMessages[1] == messages[1] {
					err := fmt.Errorf("don't return the identity and delimeter, they are added by proxy controller")
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						controller.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				controller.logger.Info("todo", "currently", "proxy redirects to the first destination", "todo", "need to direct through pipeline")
				client := controller.destination
				if client == nil {
					err := fmt.Errorf("'%s' dealer wasn't registered", messages[2])
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						controller.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				if controller.replyHandler != nil {
					err = controller.requestMessages.Add(messages[0], messages[2:])
					if err != nil {
						if replyErr := replyErrorMessage(frontend, err, messages); replyErr != nil {
							controller.logger.Fatal("reply_error_message", "error", replyErr)
						}
						continue
					}
				}

				// send the id
				_, err = client.socket.Send(messages[0], zmq.SNDMORE)
				if err != nil {
					controller.logger.Fatal("send to dealer", "error", err)
				}
				// send the delimiter
				_, err = client.socket.Send(messages[1], zmq.SNDMORE)
				if err != nil {
					controller.logger.Fatal("send to dealer", "error", err)
				}
				// skip the command name
				// we skip the router name,
				// sending the message.Request part
				lastIndex := len(destinationMessages) - 1
				for i := 0; i <= lastIndex; i++ {
					if i == lastIndex {
						_, err := client.socket.Send(destinationMessages[i], 0)
						if err != nil {
							controller.logger.Fatal("send to dealer", "error", err)
						}
					} else {
						_, err := client.socket.Send(destinationMessages[i], zmq.SNDMORE)
						if err != nil {
							controller.logger.Fatal("send to dealer", "error", err)
						}
					}
				}

				// end of handling requestMessages from source
				///////////////////////////////////////
			} else {
				for {
					messages, err := zmqSocket.RecvMessage(0)
					if err != nil {
						controller.logger.Fatal("receive from dealer", "error", err)
					}

					if controller.replyHandler != nil {
						if !controller.requestMessages.Exist(messages[0]) {
							controller.logger.Fatal("reply handler needs request messages but not found", "id", messages[0])
						}

						rawRequestMessages, err := controller.requestMessages.Take(messages[0])
						if err != nil {
							controller.logger.Fatal("failed to take the request messages", "error", err)
						}
						requestMessages, ok := rawRequestMessages.([]string)
						if !ok {
							controller.logger.Fatal("failed to decompose interfaces into the request message", "raw messages", requestMessages)
						}
						messages = controller.replyHandler(messages, requestMessages, controller.logger)
					}

					_, err = frontend.SendMessage(messages, 0)
					if err != nil {
						controller.logger.Fatal("send from dealer to frontend", "error", err)
					}
				}
			}
		}
	}
}

// The router's error replier
func replyErrorMessage(socket *zmq.Socket, newErr error, messages []string) error {
	fail := message.Fail("frontend receive message error " + newErr.Error())
	failString, _ := fail.String()

	_, err := socket.Send(messages[0], zmq.SNDMORE)
	if err != nil {
		return fmt.Errorf("failed to send back id to frontend '%s': %w", failString, err)
	}
	_, err = socket.Send(messages[1], zmq.SNDMORE)
	if err != nil {
		return fmt.Errorf("failed to send back delimiter to frontend '%s': %w", failString, err)
	}
	_, err = socket.Send(failString, 0)
	if err != nil {
		return fmt.Errorf("failed to send back fail message to frontend '%s': %w", failString, err)
	}

	return nil
}
