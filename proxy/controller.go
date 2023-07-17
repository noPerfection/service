package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/remote"
	zmq "github.com/pebbe/zmq4"

	"github.com/ahmetson/service-lib/log"
)

// HandleFunc is working over the string.
// If the string passes, then the final way is returned. Otherwise, it returns the error.
//
// Intention of the Proxy services is that, if the error is given, then message is returned back.
// Otherwise, the message directed forward to the destination clients.
//
// the first argument is the message without the identity and delimiter.
type HandleFunc = func([]string, log.Logger, []*DestinationClient, remote.Clients) ([]string, string, error)

// ControllerName is the name of the proxy for other processes
const ControllerName = "proxy_controller"

// Url defines the proxy controller path
const Url = "inproc://" + ControllerName

// DestinationClient Asynchronous Requests.
// The Dealer is the Request from Router to the
// Reply Controller.
//
// The socket.Type must be zmq.DEALER
type DestinationClient struct {
	// Could be Remote or Inproc
	Config *configuration.ControllerInstance
	// The client socket
	socket *zmq.Socket
}

// Controller The Proxy Controller that connects the multiple
// Reply Controllers together.
type Controller struct {
	destinationClients []*DestinationClient
	logger             log.Logger
	requestHandler     HandleFunc
}

// newController Returns the initiated Router with the service parameters
func newController(parent log.Logger) (*Controller, error) {
	logger, err := parent.Child("proxy_controller")
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	return &Controller{
		logger:             logger,
		destinationClients: make([]*DestinationClient, 0),
		requestHandler:     nil,
	}, nil
}

// SetRequestHandler sets the request handler.
// If the request handler succeeds then the request handler will have the final message
// in the "destination"
func (r *Controller) SetRequestHandler(handler HandleFunc) {
	r.requestHandler = handler
}

// Whether the dealer for the service is added or not.
// The service parameter should have the correct Limit or protocol type
func (r *Controller) destinationRegistered(destinationConfig *configuration.ControllerInstance) bool {
	for _, client := range r.destinationClients {
		if client.Config.Instance == destinationConfig.Instance {
			return true
		}
	}

	return false
}

// RegisterDestination Adds a new client that is connected to the Reply Controller.
// Verification of the service limit or service protocol type
// is handled on outside. As a result, it doesn't return
// any error.
// SDS Core can have unique command handlers.
func (r *Controller) RegisterDestination(destinationConfig *configuration.ControllerInstance) error {
	r.logger.Info("Adding client sockets that router will redirect", "destinationConfig", *destinationConfig)

	if r.destinationRegistered(destinationConfig) {
		return fmt.Errorf("duplicate destination instance url '%s'", destinationConfig.Instance)
	}

	destinationClient := DestinationClient{Config: destinationConfig, socket: nil}
	r.destinationClients = append(r.destinationClients, &destinationClient)
	return nil
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
func (r *Controller) setSocket(index uint64) error {
	socket, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = socket.Connect(remote.ClientUrl(r.destinationClients[index].Config.Name, r.destinationClients[index].Config.Port))
	if err != nil {
		return fmt.Errorf("setup of dealer socket: %w", err)
	}

	r.destinationClients[index].socket = socket

	return nil
}

// Returns the route to the dealer based on the command name.
// Case-sensitive.
func (r *Controller) getClient(instance string) *DestinationClient {
	for _, dealer := range r.destinationClients {
		if dealer.Config.Instance == instance {
			return dealer
		}
	}

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
func (r *Controller) Run() {
	if len(r.destinationClients) == 0 {
		r.logger.Fatal("no destinations registered in the proxy", "hint", "call router.RegisterDestination()")
	}
	if r.requestHandler == nil {
		r.logger.Fatal("request handler wasn't set")
	}
	r.logger.Info("setup the dealer sockets")
	//  Initialize poll set
	poller := zmq.NewPoller()

	// let's set the socket
	for index := range r.destinationClients {
		err := r.setSocket(uint64(index))
		if err != nil {
			r.logger.Fatal("setSocket", "destination #", index, "destination instance", r.destinationClients[index].Config.Instance)
		}
		poller.Add(r.destinationClients[index].socket, zmq.POLLIN)
	}
	r.logger.Info("dealers set up successfully")
	r.logger.Info("setup router", "url", Url)

	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer func() {
		err := frontend.Close()
		if err != nil {
			r.logger.Fatal("failed to close the socket", "error", err)
		}
	}()
	err := frontend.Bind(Url)
	if err != nil {
		r.logger.Fatal("zmq new router", "error", err)
	}
	hwm, _ := frontend.GetRcvhwm()
	r.logger.Warn("high watermark from router", hwm)

	poller.Add(frontend, zmq.POLLIN)

	r.logger.Info("The proxy controller waits for client requests")

	//  Switch messages between sockets
	for {
		// The '-1' argument indicates waiting for the
		// infinite amount of time.
		sockets, err := poller.Poll(-1)
		if err != nil {
			r.logger.Fatal("poller", "error", err)
		}
		for _, socket := range sockets {
			zmqSocket := socket.Socket
			// redirect to the dealer
			if zmqSocket == frontend {
				messages, err := zmqSocket.RecvMessage(0)
				if err != nil {
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						r.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				// Let's bypass the string
				destinationMessages, instance, err := r.requestHandler(messages[2:], r.logger, r.destinationClients, nil)
				if err != nil {
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						r.logger.Fatal("replyErrorMessage", "error", err)
					}
					continue
				}
				if len(destinationMessages) > 2 &&
					destinationMessages[0] == messages[0] &&
					destinationMessages[1] == messages[1] {
					err := fmt.Errorf("don't return the identity and delimtere, they are added by proxy controller")
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						r.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				client := r.getClient(instance)
				if client == nil {
					err := fmt.Errorf("'%s' dealer wasn't registered", messages[2])
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						r.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				// send the id
				_, err = client.socket.Send(messages[0], zmq.SNDMORE)
				if err != nil {
					r.logger.Fatal("send to dealer", "error", err)
				}
				// send the delimiter
				_, err = client.socket.Send(messages[1], zmq.SNDMORE)
				if err != nil {
					r.logger.Fatal("send to dealer", "error", err)
				}
				// skip the command name
				// we skip the router name,
				// sending the message.Request part
				lastIndex := len(destinationMessages) - 1
				for i := 0; i <= lastIndex; i++ {
					if i == lastIndex {
						_, err := client.socket.Send(destinationMessages[i], 0)
						if err != nil {
							r.logger.Fatal("send to dealer", "error", err)
						}
					} else {
						_, err := client.socket.Send(destinationMessages[i], zmq.SNDMORE)
						if err != nil {
							r.logger.Fatal("send to dealer", "error", err)
						}
					}
				}

				// end of handling requests from source
				///////////////////////////////////////
			} else {
				for {
					msg, err := zmqSocket.Recv(0)
					if err != nil {
						r.logger.Fatal("receive from dealer", "error", err)
					}
					if more, err := zmqSocket.GetRcvmore(); more {
						if err != nil {
							r.logger.Fatal("receive more messages from dealer", "error", err)
						}
						_, err := frontend.Send(msg, zmq.SNDMORE)
						if err != nil {
							r.logger.Fatal("send from dealer to frontend", "error", err)
						}
					} else {
						_, err := frontend.Send(msg, 0)
						if err != nil {
							r.logger.Fatal("send from dealer to frontend", "error", err)
						}
						break
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
