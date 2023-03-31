// This package defines the data types, and methods that interact with a remote SDS service.
//
// The request reply socket follows the Lazy Pirate pattern.
//
// Example using pebbe/zmq4 is here:
// https://github.com/pebbe/zmq4/blob/83013091510dd1275bbf0b9a302533cadc17d392/examples/lpclient.go
//
// The Lazy Pirate pattern is described in the ZMQ guide:
// https://zguide.zeromq.org/docs/chapter4/#Client-Side-Reliability-Lazy-Pirate-Pattern
package remote

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/remote/parameter"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/security/credentials"
	zmq "github.com/pebbe/zmq4"
)

// Over the socket the remote call is happening.
// This is the wrapper of zeromq socket. Wrapper enables to create larger network patterns.
type Socket struct {
	// The name of remote SDS service and its URL
	// its used as a clarification
	remote_service     *service.Service
	client_credentials *credentials.Credentials
	poller             *zmq.Poller
	socket             *zmq.Socket
	protocol           string
	inproc_url         string
	logger             log.Logger
	app_config         *configuration.Config
}

// Initiates the socket with a timeout.
// If the socket is already given, then reconnect() closes it.
// Then creates a new socket.
//
// If no socket is given, then initiates a zmq.REQ socket.
func (socket *Socket) reconnect() error {
	var socket_ctx *zmq.Context
	var socket_type zmq.Type

	if socket.socket != nil {
		ctx, err := socket.socket.Context()
		if err != nil {
			return fmt.Errorf("failed to get context from zmq socket: %w", err)
		} else {
			socket_ctx = ctx
		}

		socket_type, err = socket.socket.GetType()
		if err != nil {
			return fmt.Errorf("failed to get socket type from zmq socket: %w", err)
		}

		err = socket.Close()
		if err != nil {
			return fmt.Errorf("failed to close socket in zmq: %w", err)
		}
		socket.socket = nil
	} else {
		return fmt.Errorf("no socket initiated: %s", "reconnect")
	}

	sock, err := socket_ctx.NewSocket(socket_type)
	if err != nil {
		return fmt.Errorf("failed to create %s socket: %w", socket_type.String(), err)
	} else {
		socket.socket = sock
		err = socket.socket.SetLinger(0)
		if err != nil {
			return fmt.Errorf("failed to set up linger parameter for zmq socket: %w", err)
		}
	}

	if socket.client_credentials != nil {
		socket.client_credentials.SetClientAuthCurve(socket.socket, socket.remote_service.Credentials.PublicKey)
		if err != nil {
			return fmt.Errorf("credentials.SetClientAuthCurve: %w", err)
		}
	}

	if err := socket.socket.Connect(socket.remote_service.Url()); err != nil {
		return fmt.Errorf("socket connect: %w", err)
	}

	socket.poller = zmq.NewPoller()
	socket.poller.Add(socket.socket, zmq.POLLIN)

	return nil
}

func (socket *Socket) inproc_reconnect() error {
	var socket_ctx *zmq.Context
	var socket_type zmq.Type

	if socket.socket != nil {
		ctx, err := socket.socket.Context()
		if err != nil {
			return fmt.Errorf("failed to get context from zmq socket: %w", err)
		} else {
			socket_ctx = ctx
		}

		socket_type, err = socket.socket.GetType()
		if err != nil {
			return fmt.Errorf("failed to get socket type from zmq socket: %w", err)
		}

		err = socket.Close()
		if err != nil {
			return fmt.Errorf("failed to close socket in zmq: %w", err)
		}
		socket.socket = nil
	} else {
		return fmt.Errorf("failed to create zmq context: %s", "inproc_reconnect")
	}

	sock, err := socket_ctx.NewSocket(socket_type)
	if err != nil {
		return fmt.Errorf("failed to create %s socket: %w", socket_type.String(), err)
	} else {
		socket.socket = sock
		err = socket.socket.SetLinger(0)
		if err != nil {
			return fmt.Errorf("failed to set up linger parameter for zmq socket: %w", err)
		}
	}

	if err := socket.socket.Connect(socket.inproc_url); err != nil {
		return fmt.Errorf("error '%s' connect: %w", socket.inproc_url, err)
	}

	socket.poller = zmq.NewPoller()
	socket.poller.Add(socket.socket, zmq.POLLIN)

	return nil
}

// Close the remote connection
func (socket *Socket) Close() error {
	err := socket.socket.Close()
	if err != nil {
		return fmt.Errorf("error closing socket: %w", err)
	}

	return nil
}

// Send a command to the remote SDS service via the router.
// Both inproc and tcp
// Router identifies the redirecting rule based on service_type.
// Note that it converts the failure reply into an error. Rather than replying reply itself back to user.
// In case of successful request, the function returns reply parameters.
func (socket *Socket) RequestRouter(service_type service.ServiceType, request *message.Request) (key_value.KeyValue, error) {
	request_timeout := parameter.RequestTimeout(socket.app_config)

	request_string, err := request.ToString()
	if err != nil {
		return nil, fmt.Errorf("request.ToString: %w", err)
	}

	attempt := parameter.Attempt(socket.app_config)

	// we attempt requests for an infinite amount of time.
	for {
		//  We send a request, then we work to get a reply
		if _, err := socket.socket.SendMessage(service_type.ToString(), request_string); err != nil {
			return nil, fmt.Errorf("failed to send the command '%s' to. socket error: %w", request.Command, err)
		}

		//  Poll socket for a reply, with timeout
		sockets, err := socket.poller.Poll(request_timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to to send the command '%s'. poll error: %w", request.Command, err)
		}

		//  Here we process a server reply and exit our loop if the
		//  reply is valid. If we didn't a reply we close the client
		//  socket and resend the request. We try a number of times
		//  before finally abandoning:

		if len(sockets) > 0 {
			// Wait for reply.
			r, err := socket.socket.RecvMessage(0)
			if err != nil {
				return nil, fmt.Errorf("failed to receive the command '%s' message. socket error: %w", request.Command, err)
			}

			reply, err := message.ParseReply(r)
			if err != nil {
				return nil, fmt.Errorf("failed to parse the command '%s': %w", request.Command, err)
			}

			if !reply.IsOK() {
				return nil, fmt.Errorf("the command '%s' replied with a failure: %s", request.Command, reply.Message)
			}

			return reply.Parameters, nil
		} else {
			socket.logger.Warn("router timeout", "target service", service_type, "request_command", request.Command, "attempts_left", attempt)
			// if attempts are 0, we reconnect to remove the buffer queue.
			if socket.protocol == "inproc" {
				err := socket.inproc_reconnect()
				if err != nil {
					return nil, fmt.Errorf("socket.inproc_reconnect: %w", err)
				}
			} else {
				err := socket.reconnect()
				if err != nil {
					return nil, fmt.Errorf("socket.reconnect: %w", err)
				}
			}
			if attempt == 0 {
				return nil, fmt.Errorf("timeout")
			}
			attempt--
		}
	}
}

// Send a command to the remote SDS service. Both for inproc and tcp
// Note that it converts the failure reply into an error. Rather than replying reply itself back to user.
// In case of successful request, the function returns reply parameters.
func (socket *Socket) RequestRemoteService(request *message.Request) (key_value.KeyValue, error) {
	request_timeout := parameter.RequestTimeout(socket.app_config)

	request_string, err := request.ToString()
	if err != nil {
		return nil, fmt.Errorf("request.ToString: %w", err)
	}

	attempt := parameter.Attempt(socket.app_config)

	// we attempt requests for an infinite amount of time.
	for {
		//  We send a request, then we work to get a reply
		if _, err := socket.socket.SendMessage(request_string); err != nil {
			return nil, fmt.Errorf("failed to send the command '%s'. socket error: %w", request.Command, err)
		}

		//  Poll socket for a reply, with timeout
		sockets, err := socket.poller.Poll(request_timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to to send the command '%s'. poll error: %w", request.Command, err)
		}

		//  Here we process a server reply and exit our loop if the
		//  reply is valid. If we didn't a reply we close the client
		//  socket and resend the request. We try a number of times
		//  before finally abandoning:

		if len(sockets) > 0 {
			// Wait for reply.
			r, err := socket.socket.RecvMessage(0)
			if err != nil {
				return nil, fmt.Errorf("failed to receive the command '%s' message. socket error: %w", request.Command, err)
			}

			reply, err := message.ParseReply(r)
			if err != nil {
				return nil, fmt.Errorf("failed to parse the command '%s': %w", request.Command, err)
			}

			if !reply.IsOK() {
				return nil, fmt.Errorf("the command '%s' replied with a failure: %s", request.Command, reply.Message)
			}

			return reply.Parameters, nil
		} else {
			socket.logger.Warn("timeout", "request_command", request.Command, "attempts_left", attempt)
			if socket.protocol == "inproc" {
				err := socket.inproc_reconnect()
				if err != nil {
					return nil, fmt.Errorf("socket.inproc_reconnect: %w", err)
				}
			} else {
				err := socket.reconnect()
				if err != nil {
					return nil, fmt.Errorf("socket.reconnect: %w", err)
				}
			}

			if attempt == 0 {
				return nil, fmt.Errorf("timeout")
			}
			attempt--
		}
	}
}

// Create a new Socket on TCP protocol otherwise exit from the program
// The socket is the wrapper over zmq.REQ
func NewTcpSocket(remote_service *service.Service, client *credentials.Credentials, parent log.Logger, app_config *configuration.Config) (*Socket, error) {
	if app_config == nil {
		return nil, fmt.Errorf("missing app_config")
	}

	if remote_service == nil ||
		remote_service.IsInproc() ||
		!remote_service.IsRemote() {
		return nil, fmt.Errorf("remote service is not a remote service with REMOTE limit")
	}

	sock, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	logger, err := parent.ChildWithTimestamp("tcp_socket")
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	new_socket := Socket{
		remote_service:     remote_service,
		client_credentials: client,
		socket:             sock,
		protocol:           "tcp",
		logger:             logger,
		app_config:         app_config,
	}
	err = new_socket.reconnect()
	if err != nil {
		return nil, fmt.Errorf("reconnect: %w", err)
	}

	return &new_socket, nil
}

// Create an inter-process socket.
// Through this socket a thread could connect to
// another thread.
//
// The `url` should start with `inproc://`
func InprocRequestSocket(url string, parent log.Logger, app_config *configuration.Config) (*Socket, error) {
	if app_config == nil {
		return nil, fmt.Errorf("missing app_config")
	}

	if len(url) < 9 {
		return nil, fmt.Errorf("the url is too short")
	}
	if url[:9] != "inproc://" {
		return nil, fmt.Errorf("url doesn't start with `inproc` protocol. Its %s", url[:9])
	}

	sock, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	logger, err := parent.ChildWithTimestamp("inproc_socket")
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	new_socket := Socket{
		socket:             sock,
		protocol:           "inproc",
		inproc_url:         url,
		client_credentials: nil,
		logger:             logger,
		app_config:         app_config,
	}
	err = new_socket.inproc_reconnect()
	if err != nil {
		return nil, fmt.Errorf("new_socket.inproc_reconnect: %w", err)
	}

	return &new_socket, nil
}

// Create a new Socket on TCP protocol otherwise exit from the program
// The socket is the wrapper over zmq.SUB
func NewTcpSubscriber(e *service.Service, client *credentials.Credentials, parent log.Logger, app_config *configuration.Config) (*Socket, error) {
	if app_config == nil {
		return nil, fmt.Errorf("missing app_config")
	}
	if e == nil {
		return nil, fmt.Errorf("missing service")
	}
	if !e.IsSubscribe() || e.IsInproc() {
		return nil, fmt.Errorf("the service is a tcp or it doesn't SUBSCRIBE limit")
	}

	socket, sockErr := zmq.NewSocket(zmq.SUB)
	if sockErr != nil {
		return nil, fmt.Errorf("new sub socket: %w", sockErr)
	}

	if client != nil {
		err := client.SetClientAuthCurve(socket, e.Credentials.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("credentials.SetClientAuthCurve: %w", err)
		}
	}

	conErr := socket.Connect(e.Url())
	if conErr != nil {
		return nil, fmt.Errorf("connect to broadcast: %w", conErr)
	}

	logger, err := parent.ChildWithTimestamp("tcp_subscriber")
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	return &Socket{
		remote_service:     e,
		socket:             socket,
		client_credentials: client,
		protocol:           "tcp",
		logger:             logger,
		app_config:         app_config,
	}, nil
}
