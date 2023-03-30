package service

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/security/credentials"
)

// Environment variables for each SDS Service
type Service struct {
	Name        string // Service name
	Credentials *credentials.Credentials
	inproc      bool
	url         string
	limit       Limit
}

// Creates the service with the parameters but without any information
func Inprocess(service_type ServiceType) *Service {
	name := string(service_type)

	s := Service{
		Name:   name,
		inproc: true,
		url:    "inproc://reply_" + name,
		limit:  THIS,
	}

	return &s
}

// Creates the service with the parameters but without any information
func NewExternal(service_type ServiceType, limit Limit, app_config *configuration.Config) (*Service, error) {
	default_configuration := DefaultConfiguration(service_type)
	app_config.SetDefaults(default_configuration)

	name := string(service_type)
	host_env := name + "_HOST"
	port_env := name + "_PORT"
	broadcast_host_env := name + "_BROADCAST_HOST"
	broadcast_port_env := name + "_BROADCAST_PORT"

	s := Service{
		Name:        name,
		inproc:      false,
		Credentials: nil,
		limit:       limit,
	}

	switch limit {
	case REMOTE:
		s.url = fmt.Sprintf("tcp://%s:%s", app_config.GetString(host_env), app_config.GetString(port_env))
	case THIS:
		s.url = fmt.Sprintf("tcp://*:%s", app_config.GetString(port_env))
	case SUBSCRIBE:
		s.url = fmt.Sprintf("tcp://%s:%s", app_config.GetString(broadcast_host_env), app_config.GetString(broadcast_port_env))
	case BROADCAST:
		s.url = fmt.Sprintf("tcp://*:%s", app_config.GetString(broadcast_port_env))
	}

	return &s, nil
}

// Creates the service with the parameters that includes
// private and private key
func NewSecure(service_type ServiceType, limit Limit, app_config *configuration.Config) (*Service, error) {
	s, err := NewExternal(service_type, limit, app_config)
	if err != nil {
		return nil, fmt.Errorf("service.New: %w", err)
	}

	name := string(service_type)
	public_key := name + "_PUBLIC_KEY"
	broadcast_public_key := name + "_BROADCAST_PUBLIC_KEY"

	switch limit {
	case REMOTE:
		if !app_config.Exist(public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
		}
		s.Credentials = credentials.New(public_key)
	case THIS:
		bucket, key_name := service_type.SecretKeyPath()

		creds, err := credentials.NewFromVault(bucket, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", s.Name, err)
		}

		s.Credentials = creds
	case SUBSCRIBE:
		if !app_config.Exist(broadcast_public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
		}

		s.Credentials = credentials.New(app_config.GetString(broadcast_public_key))
	case BROADCAST:
		bucket, key_name := service_type.BroadcastSecretKeyPath()

		creds, err := credentials.NewFromVault(bucket, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", s.Name, err)
		}

		s.Credentials = creds
	}

	return s, nil
}

// Returns the request-reply url as a host:port
func (e *Service) Url() string {
	return e.url
}

// Whether the service defined for this machine as a broadcaster.
// If so, then URL host will be a '*' wildcard.
func (e *Service) IsBroadcast() bool {
	return e.limit == BROADCAST
}

// Whether the service defined for the remote broadcaster.
// If so, then URL will have host:port.
func (e *Service) IsSubscribe() bool {
	return e.limit == SUBSCRIBE
}

// Whether the service defined for the remote machine.
// If so, then URL will have host:port.
func (e *Service) IsRemote() bool {
	return e.limit == REMOTE
}

// Whether the service defined for this machine.
// If so, then URL will have * wildcard for the host.
func (e *Service) IsThis() bool {
	return e.limit == THIS
}
