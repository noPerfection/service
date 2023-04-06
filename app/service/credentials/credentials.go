package credentials

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/security/credentials"
)

// Gets the credentials for the service type
func ServiceCredentials(service_type service.ServiceType, limit service.Limit, app_config *configuration.Config) (*credentials.Credentials, error) {
	name := string(service_type)
	public_key := name + "_PUBLIC_KEY"
	broadcast_public_key := name + "_BROADCAST_PUBLIC_KEY"

	switch limit {
	case service.REMOTE:
		if !app_config.Exist(public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", name)
		}
		return credentials.New(public_key), nil
	case service.THIS:
		bucket, key_name := service_type.SecretKeyPath()

		creds, err := credentials.NewFromVault(bucket, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", name, err)
		}

		return creds, nil
	case service.SUBSCRIBE:
		if !app_config.Exist(broadcast_public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", name)
		}

		return credentials.New(app_config.GetString(broadcast_public_key)), nil
	case service.BROADCAST:
		bucket, key_name := service_type.BroadcastSecretKeyPath()

		creds, err := credentials.NewFromVault(bucket, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", name, err)
		}

		return creds, nil
	}

	return nil, fmt.Errorf("unsupported service limit: %v", limit)
}
