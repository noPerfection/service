package identity

// ServiceType defines the available kind of services
// If you are creating a new service, then define the constant value here.
type ServiceType string

const (
	// Proxy services are handling the message and redirects it to another service
	Proxy ServiceType = "proxy"
	// Extension services are exposing the API to be used by Independent, Proxy or other Extension.
	Extension ServiceType = "extension"
	// Independent service means the service is intended to be used as a standalone service
	Independent ServiceType = "root"
)
