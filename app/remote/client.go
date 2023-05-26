// Package remote defines client socket that can access to the remote service.
package remote

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/remote/parameter"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"

	// move out dependency from security/auth
	"github.com/blocklords/sds/security/auth"
	zmq "github.com/pebbe/zmq4"
)

// ClientSocket is the wrapper around zeromq's socket.
// The socket is the client's socket that will try to interact with the remote service.
type ClientSocket struct {
	// The name of remote SDS service and its URL
	// its used as a clarification
	remote_service     *service.Service
	client_credentials *auth.Credentials
	server_public_key  string
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
func (socket *ClientSocket) reconnect() error {
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
		socket.client_credentials.SetClientAuthCurve(socket.socket, socket.server_public_key)
		if err != nil {
			return fmt.Errorf("auth.SetClientAuthCurve: %w", err)
		}
	}

	if err := socket.socket.Connect(socket.remote_service.Url()); err != nil {
		return fmt.Errorf("socket connect: %w", err)
	}

	socket.poller = zmq.NewPoller()
	socket.poller.Add(socket.socket, zmq.POLLIN)

	return nil
}

// Attempts to connect to the endpoint.
// The difference from socket.reconnect() is that it will not authenticate if security is enabled.
func (socket *ClientSocket) inproc_reconnect() error {
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

// Close the socket free the port and resources.
func (socket *ClientSocket) Close() error {
	err := socket.socket.Close()
	if err != nil {
		return fmt.Errorf("error closing socket: %w", err)
	}

	return nil
}

// RequestRouter sends request message to the router. Then router will redirect it to the controller
// defined in the service_type.
//
// Supports both inproc and TCP protocols.
//
// The socket should be the router's socket.
func (socket *ClientSocket) RequestRouter(service *service.Service, request *message.Request) (key_value.KeyValue, error) {
	request_timeout := parameter.RequestTimeout(socket.app_config)

	if socket.protocol == "inproc" {
		err := socket.inproc_reconnect()
		if err != nil {
			return nil, fmt.Errorf("inproc_reconnect: %w", err)
		}

	} else {
		err := socket.reconnect()
		if err != nil {
			return nil, fmt.Errorf("reconnect: %w", err)
		}
	}

	request_string, err := request.ToString()
	if err != nil {
		return nil, fmt.Errorf("request.ToString: %w", err)
	}

	attempt := parameter.Attempt(socket.app_config)

	// we attempt requests for an infinite amount of time.
	for {
		//  We send a request, then we work to get a reply
		if _, err := socket.socket.SendMessage(service.Name, request_string); err != nil {
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
			socket.logger.Warn("Timeout! Are you sure that remote service is running?", "target service name", service.Name, "request_command", request.Command, "attempts_left", attempt, "request_timeout", request_timeout)
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

// RequestRemoteService sends the request message to the socket.
// Returns the message.Reply.Parameters in case of success.
//
// Error is returned in other cases.
//
// If the remote service returned failure message its converted into an error.
//
// The socket type should be REQ or PUSH.
func (socket *ClientSocket) RequestRemoteService(request *message.Request) (key_value.KeyValue, error) {
	if socket.protocol == "inproc" {
		err := socket.inproc_reconnect()
		if err != nil {
			return nil, fmt.Errorf("socket connection: %w", err)
		}

	} else {
		err := socket.reconnect()
		if err != nil {
			return nil, fmt.Errorf("socket connection: %w", err)
		}
	}

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

// SetSecurity will set the curve credentials in the socket to authenticate with the remote service.
func (socket *ClientSocket) SetSecurity(server_public_key string, client *auth.Credentials) *ClientSocket {
	socket.server_public_key = server_public_key
	socket.client_credentials = client

	return socket
}

// NewTcpSocket creates a new client socket over TCP protocol.
//
// The returned socket client then can send message to controller.Router and controller.Reply
func NewTcpSocket(remote_service *service.Service, parent log.Logger, app_config *configuration.Config) (*ClientSocket, error) {
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

	logger, err := parent.Child("client_socket",
		"remote_service", remote_service.Name,
		"protocol", "tcp",
		"socket_type", "REQ",
		"remote_service_url", remote_service.Url(),
	)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	new_socket := ClientSocket{
		remote_service:     remote_service,
		client_credentials: nil,
		socket:             sock,
		protocol:           "tcp",
		logger:             logger,
		app_config:         app_config,
	}

	return &new_socket, nil
}

// InprocRequestSocket creates a client socket with inproc protocol.
// The created client socket can connect to controller.Router or controller.Reply.
//
// The `url` parameter must start with `inproc://`
func InprocRequestSocket(url string, parent log.Logger, app_config *configuration.Config) (*ClientSocket, error) {
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

	logger, err := parent.Child("client_socket",
		"protocol", "inproc",
		"socket_type", "REQ",
		"remote_service_url", url)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	new_socket := ClientSocket{
		socket:             sock,
		protocol:           "inproc",
		inproc_url:         url,
		client_credentials: nil,
		logger:             logger,
		app_config:         app_config,
	}

	return &new_socket, nil
}

// NewTcpSubscriber create a new client socket on TCP protocol.
// The created client can subscribe to broadcast.Broadcast
func NewTcpSubscriber(e *service.Service, server_public_key string, client *auth.Credentials, parent log.Logger, app_config *configuration.Config) (*ClientSocket, error) {
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
		err := client.SetClientAuthCurve(socket, server_public_key)
		if err != nil {
			return nil, fmt.Errorf("client.SetClientAuthCurve: %w", err)
		}
	}

	conErr := socket.Connect(e.Url())
	if conErr != nil {
		return nil, fmt.Errorf("connect to broadcast: %w", conErr)
	}

	logger, err := parent.Child("client_socket",
		"remote_service", e.Name,
		"protocol", "tcp",
		"socket_type", "Subscriber",
		"remote_service_url", e.Url(),
	)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	return &ClientSocket{
		remote_service:     e,
		socket:             socket,
		client_credentials: client,
		server_public_key:  server_public_key,
		protocol:           "tcp",
		logger:             logger,
		app_config:         app_config,
	}, nil
}
