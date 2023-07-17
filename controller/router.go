package controller

import (
	"fmt"

	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/identity"

	"github.com/ahmetson/service-lib/log"

	zmq "github.com/pebbe/zmq4"
)

// Dealer Asynchronous Requests.
// The Dealer is the Request from Router to the
// Reply Controller.
//
// The socket.Type must be zmq.DEALER
type Dealer struct {
	// The reply controller parameter
	// Could be Remote or Inproc
	service *identity.Service
	// The client socket
	socket *zmq.Socket
}

// Router The Proxy Controller that connects the multiple
// Reply Controllers together.
type Router struct {
	service *identity.Service
	dealers []*Dealer
	logger  log.Logger
}

// NewRouter Returns the initiated Router with the service parameters
func NewRouter(service *identity.Service, parent log.Logger) (Router, error) {
	logger, err := parent.Child("controller", "type", "router", "service_name", service.Name, "inproc", service.IsInproc())
	if err != nil {
		return Router{}, fmt.Errorf("error creating child logger: %w", err)
	}

	logger.Info("New router", "service_name", service.Name)

	if service == nil || !service.IsInproc() && !service.IsThis() {
		return Router{}, fmt.Errorf("the router should be with a THIS limit or inproc type")
	}

	dealers := make([]*Dealer, 0)

	return Router{logger: logger, service: service, dealers: dealers}, nil
}

// Whether the dealer for the service is added or not.
// The service parameter should have the correct Limit or protocol type
func (r *Router) serviceRegistered(service *identity.Service) bool {
	for _, dealer := range r.dealers {
		if dealer.service.Url() == service.Url() {
			return true
		}
	}

	return false
}

// Add a new client that is connected to the Reply Controller.
// Verification of the service limit or service protocol type
// is handled on outside. As a result, it doesn't return
// any error.
func (r *Router) addService(service *identity.Service) {
	dealer := Dealer{service: service, socket: nil}
	r.dealers = append(r.dealers, &dealer)
}

// AddDealers Registers the route from command to dealer.
// SDS Core can have unique command handlers.
func (r *Router) AddDealers(services ...*identity.Service) error {
	r.logger.Info("Adding client sockets that router will redirect")

	if len(r.dealers) > 0 && r.dealers[0].socket != nil {
		return fmt.Errorf("this router is already running, add a dealers before calling router.Run()")
	}

	for _, service := range services {
		if !service.IsInproc() && !service.IsRemote() {
			return fmt.Errorf("the service '%s' is not with the REMOTE limit or inproc type", service.Name)
		}

		if r.serviceRegistered(service) {
			return fmt.Errorf("duplicate service url '%s'", service.Url())
		}
		r.addService(service)
	}
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
func (r *Router) addSocket(index uint64) error {
	socket, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = socket.Connect(r.dealers[index].service.Url())
	if err != nil {
		return fmt.Errorf("setup of dealer socket: %w", err)
	}

	r.dealers[index].socket = socket

	return nil
}

// Returns the route to the dealer based on the command name.
// Case-sensitive.
func (r *Router) getDealer(name string) *Dealer {
	for _, dealer := range r.dealers {
		if dealer.service.Name == name {
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
func (r *Router) Run() {
	if len(r.dealers) == 0 {
		r.logger.Fatal("no dealers registered in the router", "hint", "call router.AddDealers()")
	}
	r.logger.Info("setup the dealer sockets")
	//  Initialize poll set
	poller := zmq.NewPoller()

	// let's set the socket
	for index := range r.dealers {
		err := r.addSocket(uint64(index))
		if err != nil {
			r.logger.Fatal("add_socket", "dealer #", index, "url", r.dealers[index].service.Url())
		}
		poller.Add(r.dealers[index].socket, zmq.POLLIN)
	}
	r.logger.Info("dealers set up successfully")
	r.logger.Info("setup router", "service", r.service.Name, "url", r.service.Url())

	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer func() {
		err := frontend.Close()
		if err != nil {
			r.logger.Fatal("failed to close the socket", "error", err)
		}
	}()
	err := frontend.Bind(r.service.Url())
	if err != nil {
		r.logger.Fatal("zmq new router", "error", err)
	}
	hwm, _ := frontend.GetRcvhwm()
	r.logger.Warn("high watermark from router", hwm)

	poller.Add(frontend, zmq.POLLIN)

	r.logger.Info("The router waits for client requests", "service", r.service.Name, "url", r.service.Url())

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

				if len(messages) < 4 {
					err := fmt.Errorf("message is too short. It should have atleast 4 parts")
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						r.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}
				dealer := r.getDealer(messages[2])
				if dealer == nil {
					err := fmt.Errorf("'%s' dealer wasn't registered", messages[2])
					if err := replyErrorMessage(frontend, err, messages); err != nil {
						r.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				// send the id
				_, err = dealer.socket.Send(messages[0], zmq.SNDMORE)
				if err != nil {
					r.logger.Fatal("send to dealer", "error", err)
				}
				// send the delimiter
				_, err = dealer.socket.Send(messages[1], zmq.SNDMORE)
				if err != nil {
					r.logger.Fatal("send to dealer", "error", err)
				}
				// skip the command name
				// we skip the router name,
				// sending the message.Request part
				lastIndex := len(messages) - 1
				for i := 3; i <= lastIndex; i++ {
					if i == lastIndex {
						_, err := dealer.socket.Send(messages[i], 0)
						if err != nil {
							r.logger.Fatal("send to dealer", "error", err)
						}
					} else {
						_, err := dealer.socket.Send(messages[i], zmq.SNDMORE)
						if err != nil {
							r.logger.Fatal("send to dealer", "error", err)
						}
					}
				}
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
