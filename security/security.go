// The security package enables the authentication and encryption of the data
// This package depends on the "env" package. More specifically on the
// --plain argument. If this argument is not given, then package will enabled automically.
package security

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

type Security struct {
	enabled bool
	debug   bool
}

func New(debug bool) *Security {
	return &Security{
		enabled: true,
		debug:   debug,
	}
}

// Enables the authentication and encryption layer on of SDS Service connection.
// Under the hood it runs the ZAP (Zeromq Authentication Protocol).
//
// This function should be called at the beginning of the main() function.
func (s *Security) StartAuthentication() error {
	zmq.AuthSetVerbose(s.debug)
	err := zmq.AuthStart()
	if err != nil {
		return fmt.Errorf("zmq.AuthStart: %w", err)
	}

	// allow income from any ip address
	// for any domain name where this controller is running.
	zmq.AuthAllow("*")

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
