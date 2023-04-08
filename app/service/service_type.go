package service

import "fmt"

// ServiceType defines the name of the service that is defined in the SDS.
type ServiceType string

const (
	CORE              ServiceType = "CORE"              // The github.com/blocklords/sds module. It's a Router controller
	SPAGHETTI         ServiceType = "SPAGHETTI"         // The blockchain service is the gateway between blockchain networks and SDS
	CATEGORIZER       ServiceType = "CATEGORIZER"       // The categorizer service that keeps the decoded smartcontract event logs in the database
	STATIC            ServiceType = "STATIC"            // The static service keeps the smartcontracts, abis and configurations
	GATEWAY           ServiceType = "GATEWAY"           // The gateway is the service that is accessed by developers through SDK
	DEVELOPER_GATEWAY ServiceType = "DEVELOPER_GATEWAY" // The gateway is the service that is accessed by smartcontract contract developers to register new smartcontract
	READER            ServiceType = "READER"            // The service that reads the smartcontract parameters via blockchain service.
	WRITER            ServiceType = "WRITER"            // The service sends the transaction to the blockchain via blockchain service.
	BUNDLE            ServiceType = "BUNDLE"            // The service turns all transactions into one and then sends them to WRITER service.

	BUCKET string = "SDS_SERVICES" // Vault parameter
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
