package controller

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/service"

	zmq "github.com/pebbe/zmq4"
)

// Add whitelisted services
func WhitelistAccess(logger log.Logger, spaghetti_env *service.Service, public_keys []string) {
	logger.Info("get the whitelisted services")

	// We set the whitelisted accounts that has access to this controller
	zmq.AuthCurveAdd(spaghetti_env.Name, public_keys...)

	logger.Info("get the whitelisted subscribers")
}

// Set the private key, so connected clients can identify this controller
// You call it before running the controller
func (c *Controller) SetControllerPrivateKey() error {
	err := c.service.Credentials.SetSocketAuthCurve(c.socket, c.service.Name)
	if err == nil {
		return nil
	}
	return fmt.Errorf("ServerAuthCurve for domain %s: %w", c.service.Name, err)
}
