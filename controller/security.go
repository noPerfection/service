package controller

import (
	"github.com/ahmetson/service-lib/identity"
	"github.com/ahmetson/service-lib/log"

	// todo
	// move out security/auth dependency
	// "github.com/ahmetson/service-lib/security/auth"

	zmq "github.com/pebbe/zmq4"
)

// WhitelistAccess Adds whitelisted services
func WhitelistAccess(logger log.Logger, blockchainEnv *identity.Service, publicKeys []string) {
	logger.Info("get the whitelisted services")

	// We set the whitelisted accounts that has access to this controller
	zmq.AuthCurveAdd(blockchainEnv.Name, publicKeys...)

	logger.Info("get the whitelisted subscribers")
}

// // Set the private key, so connected clients can identify this controller
// // You call it before running the controller
// func (c *Controller) SetControllerPrivateKey(service_credentials *auth.Credentials) error {
// 	err := service_credentials.SetSocketAuthCurve(c.socket, c.service.Name)
// 	if err == nil {
// 		return nil
// 	}
// 	return fmt.Errorf("ServerAuthCurve for domain %s: %w", c.service.Name, err)
// }
