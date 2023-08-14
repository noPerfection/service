package handler

import (
	"fmt"
	client "github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/config/service"

	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/communication/command"

	zmq "github.com/pebbe/zmq4"
)

// Controller is the socket wrapper for the service.
type Controller struct {
	config             *service.Controller
	serviceUrl         string
	socket             *zmq.Socket
	logger             *log.Logger
	controllerType     service.ControllerType
	routes             *command.Routes
	requiredExtensions []string
	extensionConfigs   key_value.KeyValue
	extensions         client.Clients
}

// AddConfig adds the parameters of the handler from the config.
// The serviceUrl is the service to which this handler belongs too.
func (c *Controller) AddConfig(controller *service.Controller, serviceUrl string) {
	c.config = controller
	c.serviceUrl = serviceUrl
}

// AddExtensionConfig adds the config of the extension that the handler depends on
func (c *Controller) AddExtensionConfig(extension *service.Extension) {
	c.extensionConfigs.Set(extension.Url, extension)
}

// RequireExtension marks the extensions that this handler depends on.
// Before running, the required extension should be added from the config.
// Otherwise, handler won't run.
func (c *Controller) RequireExtension(name string) {
	c.requiredExtensions = append(c.requiredExtensions, name)
}

// RequiredExtensions returns the list of extension names required by this handler
func (c *Controller) RequiredExtensions() []string {
	return c.requiredExtensions
}

func (c *Controller) isReply() bool {
	return c.controllerType == service.SyncReplierType
}

// A reply sends to the caller the message.
//
// If handler doesn't support replying (for example, PULL handler),
// then it returns success.
func (c *Controller) reply(socket *zmq.Socket, message message.Reply) error {
	if !c.isReply() {
		return nil
	}

	reply, _ := message.String()
	if _, err := socket.SendMessage(reply); err != nil {
		return fmt.Errorf("recv error replying error %w" + err.Error())
	}

	return nil
}

// Calls handler.reply() with the error message.
func (c *Controller) replyError(socket *zmq.Socket, err error) error {
	request := message.Request{}
	return c.reply(socket, request.Fail(err.Error()))
}

// AddRoute adds a command along with its handler to this handler
func (c *Controller) AddRoute(route *command.Route) error {
	if c.routes.Exist(route.Command) {
		return nil
	}

	err := c.routes.Add(route.Command, route)
	if err != nil {
		return fmt.Errorf("failed to add a route: %w", err)
	}

	return nil
}

// extensionsAdded checks that the required extensions are added into the handler.
// If no extensions are added by calling handler.RequireExtension(), then it will return nil.
func (c *Controller) extensionsAdded() error {
	for _, name := range c.requiredExtensions {
		if err := c.extensionConfigs.Exist(name); err != nil {
			return fmt.Errorf("required '%s' extension. but it wasn't added to the handler (does it exist in config.yml?)", name)
		}
	}

	return nil
}

func (c *Controller) ControllerType() service.ControllerType {
	return c.controllerType
}

// initExtensionClients will set up the extension clients for this handler.
// It will be called by c.Run(), automatically.
//
// The reason for why we call it by c.Run() is due to the thread-safety.
//
// The handler is intended to be called as the goroutine. And if the sockets
// are not initiated within the same goroutine, then zeromq doesn't guarantee the socket work
// as it's intended.
func (c *Controller) initExtensionClients() error {
	for _, extensionInterface := range c.extensionConfigs {
		extensionConfig := extensionInterface.(*service.Extension)
		extension, err := client.NewReq(extensionConfig.Url, extensionConfig.Port, c.logger)
		if err != nil {
			return fmt.Errorf("failed to create a request client: %w", err)
		}
		c.extensions.Set(extensionConfig.Url, extension)
	}

	return nil
}

func (c *Controller) Close() error {
	if c.socket == nil {
		return nil
	}

	err := c.socket.Close()
	if err != nil {
		return fmt.Errorf("handler.socket.Close: %w", err)
	}

	return nil
}

// Url creates url of the handler url for binding.
// For clients to connect to this url, call client.ClientUrl()
func Url(name string, port uint64) string {
	if port == 0 {
		return fmt.Sprintf("inproc://%s", name)
	}
	url := fmt.Sprintf("tcp://*:%d", port)
	return url
}
