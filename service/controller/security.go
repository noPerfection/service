package controller

import (
	"fmt"

	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/parameter"

	// move out security/auth dependency
	"github.com/blocklords/sds/security/auth"

	zmq "github.com/pebbe/zmq4"
)

// Add whitelisted services
func WhitelistAccess(logger log.Logger, blockchain_env *parameter.Service, public_keys []string) {
	logger.Info("get the whitelisted services")

	// We set the whitelisted accounts that has access to this controller
	zmq.AuthCurveAdd(blockchain_env.Name, public_keys...)

	logger.Info("get the whitelisted subscribers")
}

// Set the private key, so connected clients can identify this controller
// You call it before running the controller
func (c *Controller) SetControllerPrivateKey(service_credentials *auth.Credentials) error {
	err := service_credentials.SetSocketAuthCurve(c.socket, c.service.Name)
	if err == nil {
		return nil
	}
	return fmt.Errorf("ServerAuthCurve for domain %s: %w", c.service.Name, err)
}
