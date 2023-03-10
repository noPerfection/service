package controller

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/account"
	"github.com/blocklords/sds/app/service"

	zmq "github.com/pebbe/zmq4"
)

// We set the whitelisted accounts that has access to this controller
func add_whitelisted_access(s *service.Service, accounts account.Accounts) {
	zmq.AuthCurveAdd(s.Name, accounts.PublicKeys()...)
}

// Add whitelisted services
func WhitelistAccess(logger log.Logger, spaghetti_env *service.Service, accounts account.Accounts) {
	logger.Info("get the whitelisted services")

	add_whitelisted_access(spaghetti_env, accounts)

	logger.Info("get the whitelisted subscribers")
}

// Set the private key, so connected clients can identify this controller
// You call it before running the controller
func (c *Controller) SetControllerPrivateKey() error {
	err := c.socket.ServerAuthCurve(c.service.Name, c.service.SecretKey)
	if err == nil {
		return nil
	}
	return fmt.Errorf("ServerAuthCurve for domain %s: %w", c.service.Name, err)
}
