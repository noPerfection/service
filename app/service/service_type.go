package service

import "fmt"

type ServiceType string

const (
	CORE              ServiceType = "CORE"
	SPAGHETTI         ServiceType = "SPAGHETTI"
	CATEGORIZER       ServiceType = "CATEGORIZER"
	STATIC            ServiceType = "STATIC"
	GATEWAY           ServiceType = "GATEWAY"
	DEVELOPER_GATEWAY ServiceType = "DEVELOPER_GATEWAY"
	READER            ServiceType = "READER"
	WRITER            ServiceType = "WRITER"
	BUNDLE            ServiceType = "BUNDLE"

	BUCKET string = "SDS_SERVICES"
)

// Returns the string represantion of the service type
func (s ServiceType) ToString() string {
	return string(s)
}

// Returns the Vault secret storage and the key for curve private part.
func (name ServiceType) SecretKeyPath() (string, string) {
	return BUCKET, name.ToString() + "_SECRET_KEY"
}

// Returns the Vault secret path and the key for curve private part for broadcaster.
func (name ServiceType) BroadcastSecretKeyPath() (string, string) {
	return BUCKET, name.ToString() + "_BROADCAST_SECRET_KEY"
}

func (s ServiceType) valid() error {
	types := service_types()
	for _, service_type := range types {
		if service_type == s {
			return nil
		}
	}

	return fmt.Errorf("the '%s' service type not registered", s.ToString())
}

func service_types() []ServiceType {
	return []ServiceType{
		CORE,
		SPAGHETTI,
		CATEGORIZER,
		STATIC,
		GATEWAY,
		DEVELOPER_GATEWAY,
		READER,
		WRITER,
		BUNDLE,
	}
}
