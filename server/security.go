package server

import (
	"github.com/ahmetson/log-lib"

	// todo
	// move out security/auth dependency
	// "github.com/ahmetson/service-lib/security/auth"

	zmq "github.com/pebbe/zmq4"
)

// WhitelistAccess Adds assisted services
func WhitelistAccess(logger log.Logger, domain string, publicKeys []string) {
	logger.Info("get the whitelisted services")

	// We set the assisted accounts that have access to this server
	zmq.AuthCurveAdd(domain, publicKeys...)

	logger.Info("get the whitelisted subscribers")
}

// // Set the private key, so connected clients can identify this server
// // You call it before running the server
// func (c *ControllerCategory) SetControllerPrivateKey(service_credentials *auth.Credentials) error {
// 	err := service_credentials.SetSocketAuthCurve(c.socket, c.service.Url)
// 	if err == nil {
// 		return nil
// 	}
// 	return fmt.Errorf("ServerAuthCurve for domain %s: %w", c.service.Url, err)
// }
