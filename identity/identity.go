package identity

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/service"
)

// Service defines the parameters of the service.
type Service struct {
	ServiceType service.Type
	Name        string // Service name
	inproc      bool
	url         string
	limit       Limit
}

// Inprocess creates the service with the parameters but without any information
func Inprocess(serviceType service.Type) (*Service, error) {
	if inprocErr := service.ValidateServiceType(serviceType); inprocErr != nil {
		return nil, fmt.Errorf("valid or inproc_valid: %w", inprocErr)
	}
	name := serviceType.String()

	s := Service{
		ServiceType: serviceType,
		Name:        name,
		inproc:      true,
		url:         "inproc://SERVICE_" + name,
		limit:       THIS,
	}

	return &s, nil
}

// InprocessFromUrl Creates the inprocess service by url.
// The name of the service is custom. With this function you can create
// services that are not registered in the service type.
//
// Url should include inproc:// protocol prefix

// NewExternal creates the service with the parameters but without any information
func NewExternal(serviceType service.Type, limit Limit, appConfig *configuration.Config) (*Service, error) {
	if appConfig == nil {
		return nil, fmt.Errorf("missing app config")
	}

	if err := service.ValidateServiceType(serviceType); err != nil {
		return nil, fmt.Errorf("valid: %w", err)
	}

	defaultConfiguration := DefaultConfiguration(serviceType)
	appConfig.SetDefaults(defaultConfiguration)

	name := serviceType.String()
	hostEnv := name + "_HOST"
	portEnv := name + "_PORT"
	broadcastHostEnv := name + "_BROADCAST_HOST"
	broadcastPortEnv := name + "_BROADCAST_PORT"

	s := Service{
		ServiceType: serviceType,
		Name:        name,
		inproc:      false,
		limit:       limit,
	}

	switch limit {
	case REMOTE:
		s.url = fmt.Sprintf("tcp://%s:%s", appConfig.GetString(hostEnv), appConfig.GetString(portEnv))
	case THIS:
		s.url = fmt.Sprintf("tcp://*:%s", appConfig.GetString(portEnv))
	case SUBSCRIBE:
		s.url = fmt.Sprintf("tcp://%s:%s", appConfig.GetString(broadcastHostEnv), appConfig.GetString(broadcastPortEnv))
	case BROADCAST:
		s.url = fmt.Sprintf("tcp://*:%s", appConfig.GetString(broadcastPortEnv))
	default:
		return nil, fmt.Errorf("unsupported limit: %v", limit)
	}

	return &s, nil
}

// Url Returns the endpoint of the service.
// The sockets are using this parameter.
func (e *Service) Url() string {
	return e.url
}

// IsBroadcast defines whether the service defined for this machine as a broadcaster.
// If so, then URL host will be a '*' wildcard.
func (e *Service) IsBroadcast() bool {
	return e.limit == BROADCAST && !e.inproc
}

// IsSubscribe defines whether the service defined for the remote broadcaster.
// If so, then URL will have host:port.
func (e *Service) IsSubscribe() bool {
	return e.limit == SUBSCRIBE && !e.inproc
}

// IsRemote defines whether the service defined for the remote machine.
// If so, then URL will have host:port.
func (e *Service) IsRemote() bool {
	return e.limit == REMOTE && !e.inproc
}

// IsThis defines whether the service defined for this machine.
// If so, then URL will have * wildcard for the host.
func (e *Service) IsThis() bool {
	return e.limit == THIS && !e.inproc
}

// IsInproc defines whether the protocol of service is "inproc".
// In case of "tcp" protocol it will return false.
func (e *Service) IsInproc() bool {
	return e.inproc
}
