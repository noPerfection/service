package controller

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/security/credentials"

	zmq "github.com/pebbe/zmq4"
)

// Add whitelisted services
func WhitelistAccess(logger log.Logger, spaghetti_env *service.Service, credentials []*credentials.Credentials) {
	logger.Info("get the whitelisted services")

	public_keys := make([]string, len(credentials))
	for i, k := range credentials {
		public_keys[i] = k.PublicKey
	}

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
