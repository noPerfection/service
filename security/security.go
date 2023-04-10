// The security package enables the authentication and runs the vault.
// The credentials are stored in the vault, and they will be fetched from the vault.
//
// To enable it in the app, pass --security argument.
package security

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/security/vault"
	zmq "github.com/pebbe/zmq4"
)

// Security handles the metadata about security layer.
type Security struct {
	app_config *configuration.Config
	logger     log.Logger
}

// New security with the given metadata
func New(app_config *configuration.Config, parent log.Logger) (*Security, error) {
	logger, err := parent.ChildWithTimestamp("security")
	if err != nil {
		return nil, fmt.Errorf("logger.Child: %w", err)
	}
	return &Security{
		app_config: app_config,
		logger:     logger,
	}, nil
}

// Run the security layer:
//   - authentication layer (of zeromq)
//   - vault engine
//
// If it fails to start the engine, then it will exit from app with error message.
func (s *Security) Run() {
	if err := s.start_auth(); err != nil {
		s.logger.Fatal("auth layer start", "error", err)
	}

	s.app_config.SetDefaults(vault.VaultConfigurations)
	s.app_config.SetDefaults(vault.DatabaseVaultConfigurations)

	v, err := vault.New(s.app_config, s.logger)
	if err != nil {
		s.logger.Fatal("vault.New", "error", err)
	}

	go v.PeriodicallyRenewLeases()
	go v.RunController()
}

// Enables the authentication and encryption layer on of SDS Service connection.
// Under the hood it runs the ZAP (Zeromq Authentication Protocol).
//
// This function should be called at the beginning of the main() function.
func (s *Security) start_auth() error {
	zmq.AuthSetVerbose(s.app_config.DebugSecurity)
	err := zmq.AuthStart()
	if err != nil {
		return fmt.Errorf("zmq.AuthStart: %w", err)
	}

	// allow income from any ip address
	// for any domain of this application.
	zmq.AuthAllow("*")

	// Retreive the public key of the authenticator.
	// The public key is then used by app/account and security/auth
	handler := func(version string, request_id string, domain string, address string, identity string, mechanism string, credentials ...string) (metadata map[string]string) {
		metadata = map[string]string{
			"request_id": request_id,
			"Identity":   zmq.Z85encode(credentials[0]),
			"address":    address,
			"pub_key":    zmq.Z85encode(credentials[0]), // if mechanism is not curve, it will fail
		}
		return metadata
	}
	zmq.AuthSetMetadataHandler(handler)

	return nil
}
