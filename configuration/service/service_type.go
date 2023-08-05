package service

import "fmt"

// ServiceType defines the available kind of services
// If you are creating a new service, then define the constant value here.
type ServiceType string

const (
	// ProxyType services are handling the message and redirects it to another service
	ProxyType ServiceType = "Proxy"
	// ExtensionType services are exposing the API to be used by Independent, Proxy or other Extension.
	ExtensionType ServiceType = "Extension"
	// IndependentType service means the service is intended to be used as a standalone service
	IndependentType ServiceType = "Independent"
)

// ValidateServiceType checks whether the given string is the valid or not.
// If not valid then returns the error otherwise returns nil.
func ValidateServiceType(t ServiceType) error {
	if t == ProxyType || t == ExtensionType || t == IndependentType {
		return nil
	}

	return fmt.Errorf("'%s' is not valid service type", t)
}

func (s ServiceType) String() string {
	return string(s)
}
