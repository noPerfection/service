package service

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
)

// Returns the string represantion of the service type
func (s ServiceType) ToString() string {
	return string(s)
}

// Returns the Vault secret storage and the key for curve private part.
func (name ServiceType) SecretKeyPath() (string, string) {
	return "SDS_SERVICES", name.ToString() + "_SECRET_KEY"
}

// Returns the Vault secret path and the key for curve private part for broadcaster.
func (name ServiceType) BroadcastSecretKeyPath() (string, string) {
	return "SDS_SERVICES", name.ToString() + "_BROADCAST_SECRET_KEY"
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
