// The security package enables the authentication and encryption of the data
// This package depends on the "env" package. More specifically on the
// --plain argument. If this argument is not given, then package will enabled automically.
package security

import (
	"github.com/blocklords/gosds/app/argument"

	zmq "github.com/pebbe/zmq4"
)

// Enables the authentication and encryption of SDS Service connection.
// Under the hood it runs through the ZAP (Zeromq Authentication Protocol).
//
// This function should be called at the beginning of the main() function.
func EnableSecurity() error {
	exist, err := argument.Exist(argument.PLAIN)
	if err != nil {
		return err
	}
	// Plain connection, therefore we don't start authentication
	if exist {
		return nil
	}

	debug, err := argument.Exist(argument.SECURITY_DEBUG)
	if err != nil {
		return err
	}
	zmq.AuthSetVerbose(debug)
	err = zmq.AuthStart()
	if err != nil {
		return err
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
