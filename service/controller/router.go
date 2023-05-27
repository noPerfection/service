package controller

import (
	"fmt"

	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/parameter"

	"github.com/blocklords/sds/app/log"

	zmq "github.com/pebbe/zmq4"
)

// Asynchronous Requests.
// The Dealer is the Requst from Router to the
// Reply Controller.
//
// The socket.Type must be zmq.DEALER
type Dealer struct {
	// The reply controller parameter
	// Could be Remote or Inproc
	service *parameter.Service
	// The client socket
	socket *zmq.Socket
}

// The Proxy Controller that connects the multiple
// Reply Controllers together.
type Router struct {
	service *parameter.Service
	dealers []*Dealer
	logger  log.Logger
}

// Returns the initiated Router whith the service parameters
func NewRouter(service *parameter.Service, parent log.Logger) (Router, error) {
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
func (r *Router) service_registered(service *parameter.Service) bool {
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
func (r *Router) add_service(service *parameter.Service) {
	dealer := Dealer{service: service, socket: nil}
	r.dealers = append(r.dealers, &dealer)
}

// Registers the route from command to dealer.
// SDS Core can have unique command handlers.
func (router *Router) AddDealers(services ...*parameter.Service) error {
	router.logger.Info("Adding client sockets that router will redirect")

	if len(router.dealers) > 0 && router.dealers[0].socket != nil {
		return fmt.Errorf("this router is already running, add a dealers before calling router.Run()")
	}

	for _, service := range services {
		if !service.IsInproc() && !service.IsRemote() {
			return fmt.Errorf("the service '%s' is not with the REMOTE limit or inproc type", service.Name)
		}

		if router.service_registered(service) {
			return fmt.Errorf("duplicate service url '%s'", service.Url())
		}
		router.add_service(service)
	}
	return nil
}

// Internal function that assigns the socket
// to the Clients.
//
// Its handled in this not in the the socket.
// Because called from the router goroutine (go router.Run())
//
// If the Router creating thread calls
// then as thread-unsafety, will lead to the unexpected
// behaviours.
func (router *Router) add_socket(index uint64) error {
	socket, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = socket.Connect(router.dealers[index].service.Url())
	if err != nil {
		return fmt.Errorf("setup of dealer socket: %w", err)
	}

	router.dealers[index].socket = socket

	return nil
}

// Returns the route to the dealer based on the command name.
// Case sensitive.
func (router *Router) get_dealer(name string) *Dealer {
	for _, dealer := range router.dealers {
		if dealer.service.Name == name {
			return dealer
		}
	}

	return nil
}

// Runs the router (asynchronous zmq.REP) along with dealers (asynchronous zmq.REQ).
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
//		2 - string (gosds/app/parameter.ServiceType) service name as a tag of dealer.
//	     to identify which dealer to use
//		3 - gosds/app/remote/message.Request the message that is redirected to the zmq.REP controller
//
// The request id is used to identify the client. Once the dealer gets the reply from zmq.REP controller
// the router will return it back to the client by request id.
//
// Example:
//
//	// route the msg[3] to the SDS Storage
//	msg := [0: "uid-123", 1: "", 2: "storage", 3: "{`command`: `smartcontract_get`, `parameters`: {}}"]
func (router *Router) Run() {
	if len(router.dealers) == 0 {
		router.logger.Fatal("no dealers registered in the router", "hint", "call router.AddDealers()")
	}
	router.logger.Info("setup the dealer sockets")
	//  Initialize poll set
	poller := zmq.NewPoller()

	// let's set the socket
	for index := range router.dealers {
		err := router.add_socket(uint64(index))
		if err != nil {
			router.logger.Fatal("add_socket", "dealer #", index, "url", router.dealers[index].service.Url())
		}
		poller.Add(router.dealers[index].socket, zmq.POLLIN)
	}
	router.logger.Info("dealers set up successfully")
	router.logger.Info("setup router", "service", router.service.Name, "url", router.service.Url())

	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer frontend.Close()
	err := frontend.Bind(router.service.Url())
	if err != nil {
		router.logger.Fatal("zmq new router", "error", err)
	}
	hwm, _ := frontend.GetRcvhwm()
	router.logger.Warn("high watermark from router", hwm)

	poller.Add(frontend, zmq.POLLIN)

	router.logger.Info("The router waits for client requests", "service", router.service.Name, "url", router.service.Url())

	//  Switch messages between sockets
	for {
		// The '-1' argument indicates waiting for the
		// infinite amount of time.
		sockets, err := poller.Poll(-1)
		if err != nil {
			router.logger.Fatal("poller", "error", err)
		}
		for _, socket := range sockets {
			zmq_socket := socket.Socket
			// redirect to the dealer
			if zmq_socket == frontend {
				msgs, err := zmq_socket.RecvMessage(0)
				if err != nil {
					if err := reply_error_message(frontend, err, msgs); err != nil {
						router.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				if len(msgs) < 4 {
					err := fmt.Errorf("message is too short. It should have atleast 4 parts")
					if err := reply_error_message(frontend, err, msgs); err != nil {
						router.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}
				dealer := router.get_dealer(msgs[2])
				if dealer == nil {
					err := fmt.Errorf("'%s' dealer wasn't registered", msgs[2])
					if err := reply_error_message(frontend, err, msgs); err != nil {
						router.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				// send the id
				_, err = dealer.socket.Send(msgs[0], zmq.SNDMORE)
				if err != nil {
					router.logger.Fatal("send to dealer", "error", err)
				}
				// send the delimiter
				_, err = dealer.socket.Send(msgs[1], zmq.SNDMORE)
				if err != nil {
					router.logger.Fatal("send to dealer", "error", err)
				}
				// skip the command name
				// we skip the router name,
				// sending the message.Request part
				last_index := len(msgs) - 1
				for i := 3; i <= last_index; i++ {
					if i == last_index {
						_, err := dealer.socket.Send(msgs[i], 0)
						if err != nil {
							router.logger.Fatal("send to dealer", "error", err)
						}
					} else {
						_, err := dealer.socket.Send(msgs[i], zmq.SNDMORE)
						if err != nil {
							router.logger.Fatal("send to dealer", "error", err)
						}
					}
				}
			} else {
				for {
					msg, err := zmq_socket.Recv(0)
					if err != nil {
						router.logger.Fatal("receive from dealer", "error", err)
					}
					if more, err := zmq_socket.GetRcvmore(); more {
						if err != nil {
							router.logger.Fatal("receive more messages from dealer", "error", err)
						}
						_, err := frontend.Send(msg, zmq.SNDMORE)
						if err != nil {
							router.logger.Fatal("send from dealer to frontend", "error", err)
						}
					} else {
						_, err := frontend.Send(msg, 0)
						if err != nil {
							router.logger.Fatal("send from dealer to frontend", "error", err)
						}
						break
					}
				}
			}
		}
	}
}

// The router's error replier
func reply_error_message(socket *zmq.Socket, new_err error, msgs []string) error {
	fail := message.Fail("frontend receive message error " + new_err.Error())
	fail_string, _ := fail.ToString()

	_, err := socket.Send(msgs[0], zmq.SNDMORE)
	if err != nil {
		return fmt.Errorf("failed to send back id to frontend '%s': %w", fail_string, err)
	}
	_, err = socket.Send(msgs[1], zmq.SNDMORE)
	if err != nil {
		return fmt.Errorf("failed to send back delimiter to frontend '%s': %w", fail_string, err)
	}
	_, err = socket.Send(fail_string, 0)
	if err != nil {
		return fmt.Errorf("failed to send back fail message to frontend '%s': %w", fail_string, err)
	}

	return nil
}
