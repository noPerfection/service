// Package auth defines the functions that get's the CURVE key pair of the service
// for authentication
package auth

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/parameter"

	// move out dependency from security/auth and security/vault
	"github.com/blocklords/sds/security/auth"
	"github.com/blocklords/sds/security/vault"
)

// The vault bucket name where we keep the service's curve private keys.
const BUCKET string = "SDS_SERVICES"

// Returns the Vault secret path for the service private key.
func vault_path(name parameter.ServiceType) string {
	return name.ToString() + "_SECRET_KEY"
}

// Returns the Vault secret path for the broadcast service private key.
func vault_broadcast_path(name parameter.ServiceType) string {
	return name.ToString() + "_BROADCAST_SECRET_KEY"
}

// Gets the credentials for the service type
func ServiceCredentials(service_type parameter.ServiceType, limit parameter.Limit, app_config *configuration.Config) (*auth.Credentials, error) {
	name := string(service_type)
	public_key := name + "_PUBLIC_KEY"
	broadcast_public_key := name + "_BROADCAST_PUBLIC_KEY"

	switch limit {
	case parameter.REMOTE:
		if !app_config.Exist(public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", name)
		}
		return auth.New(public_key), nil
	case parameter.THIS:
		key_name := vault_path(service_type)

		creds, err := vault.GetCredentials(BUCKET, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", name, err)
		}

		return creds, nil
	case parameter.SUBSCRIBE:
		if !app_config.Exist(broadcast_public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", name)
		}

		return auth.New(app_config.GetString(broadcast_public_key)), nil
	case parameter.BROADCAST:
		key_name := vault_broadcast_path(service_type)

		creds, err := vault.GetCredentials(BUCKET, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", name, err)
		}

		return creds, nil
	}

	return nil, fmt.Errorf("unsupported service limit: %v", limit)
}
