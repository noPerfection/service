package service

import (
	"fmt"
	"net/url"

	"github.com/blocklords/sds/app/configuration"
)

// Environment variables for each SDS Service
type Service struct {
	Name   string // Service name
	inproc bool
	url    string
	limit  Limit
}

// Creates the service with the parameters but without any information
func Inprocess(service_type ServiceType) (*Service, error) {
	if err := service_type.valid(); err != nil {
		return nil, fmt.Errorf("valid: %w", err)
	}
	name := service_type.ToString()

	s := Service{
		Name:   name,
		inproc: true,
		url:    "inproc://SERVICE_" + name,
		limit:  THIS,
	}

	return &s, nil
}

// Creates the inprocess service by url.
// The name of the service is custom. With this function you can create
// services that are not registered in the service type.
//
// Url should include inproc:// protocol prefix
func InprocessFromUrl(endpoint string) (*Service, error) {
	u, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint '%s': %w", endpoint, err)
	}
	if u.Scheme != "inproc" {
		return nil, fmt.Errorf("invalid '%s' provider protocol. Expected either 'inproc'. But given '%s'", endpoint, u.Scheme)
	}

	s := Service{
		Name:   u.Hostname(),
		inproc: true,
		url:    endpoint,
		limit:  THIS,
	}

	return &s, nil
}

// Creates the service with the parameters but without any information
func NewExternal(service_type ServiceType, limit Limit, app_config *configuration.Config) (*Service, error) {
	if app_config == nil {
		return nil, fmt.Errorf("missing app config")
	}

	if err := service_type.valid(); err != nil {
		return nil, fmt.Errorf("valid: %w", err)
	}

	default_configuration := DefaultConfiguration(service_type)
	app_config.SetDefaults(default_configuration)

	name := string(service_type)
	host_env := name + "_HOST"
	port_env := name + "_PORT"
	broadcast_host_env := name + "_BROADCAST_HOST"
	broadcast_port_env := name + "_BROADCAST_PORT"

	s := Service{
		Name:   name,
		inproc: false,
		limit:  limit,
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
	default:
		return nil, fmt.Errorf("unsupported limit: %v", limit)
	}

	return &s, nil
}

// Returns the request-reply url as a host:port
func (e *Service) Url() string {
	return e.url
}

// Whether the service defined for this machine as a broadcaster.
// If so, then URL host will be a '*' wildcard.
func (e *Service) IsBroadcast() bool {
	return e.limit == BROADCAST && !e.inproc
}

// Whether the service defined for the remote broadcaster.
// If so, then URL will have host:port.
func (e *Service) IsSubscribe() bool {
	return e.limit == SUBSCRIBE && !e.inproc
}

// Whether the service defined for the remote machine.
// If so, then URL will have host:port.
func (e *Service) IsRemote() bool {
	return e.limit == REMOTE && !e.inproc
}

// Whether the service defined for this machine.
// If so, then URL will have * wildcard for the host.
func (e *Service) IsThis() bool {
	return e.limit == THIS && !e.inproc
}

func (e *Service) IsInproc() bool {
	return e.inproc
}
