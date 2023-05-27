package service

import "fmt"

// ServiceType defines the name of the service that is defined in the SDS.
// If you are creating a new service, then define the constant value here.
type ServiceType string

const (
	CORE              ServiceType = "CORE"              // The github.com/blocklords/sds module. It's a Router controller
	BLOCKCHAIN        ServiceType = "BLOCKCHAIN"        // The blockchain service is the gateway between blockchain networks and SDS
	INDEXER           ServiceType = "INDEXER"           // The indexer service that keeps the decoded smartcontract event logs in the database
	STATIC            ServiceType = "STATIC"            // The static service keeps the smartcontracts, abis and configurations
	GATEWAY           ServiceType = "GATEWAY"           // The gateway is the service that is accessed by developers through SDK
	DEVELOPER_GATEWAY ServiceType = "DEVELOPER_GATEWAY" // The gateway is the service that is accessed by smartcontract contract developers to register new smartcontract
	READER            ServiceType = "READER"            // The service that reads the smartcontract parameters via blockchain service.
	WRITER            ServiceType = "WRITER"            // The service sends the transaction to the blockchain via blockchain service.
	BUNDLE            ServiceType = "BUNDLE"            // The service turns all transactions into one and then sends them to WRITER service.
	EVM               ServiceType = "EVM"               // The service that handles EVM blockchains.
	IMX               ServiceType = "IMX"               // The service that handles IMX blockchains.

	// The services to be used within application.
	// Don't call them on TCP protocol.
	DATABASE ServiceType = "DATABASE" // The service of database operations. Intended to be on inproc protocol.
	SECURITY ServiceType = "SECURITY" // The service of authentication and vault starter. Intended to be on inproc protocol.
	VAULT    ServiceType = "VAULT"    // The service of
)

// NewServiceType converts the string into service type and validates it.
// Validation includes checking of of [ServiceType.valid].
func NewServiceType(str string) (ServiceType, error) {
	service_type := ServiceType(str)
	if err := service_type.valid(); err != nil {
		return "", fmt.Errorf("service_type.valid() failed: %v", err)
	}

	return service_type, nil
}

// ToString returns the string represantion of the service type
func (s ServiceType) ToString() string {
	return string(s)
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

func (s ServiceType) inproc_valid() error {
	types := inproc_service_types()
	for _, service_type := range types {
		if service_type == s {
			return nil
		}
	}

	return fmt.Errorf("the '%s' service type not registered", s.ToString())
}

// Returns the services that are available for use within application only
func inproc_service_types() []ServiceType {
	return []ServiceType{
		DATABASE,
		SECURITY,
		VAULT,
	}
}

// Returns all registered services for TCP connection
func service_types() []ServiceType {
	return []ServiceType{
		CORE,
		BLOCKCHAIN,
		INDEXER,
		STATIC,
		GATEWAY,
		DEVELOPER_GATEWAY,
		READER,
		WRITER,
		BUNDLE,
		EVM,
		IMX,
	}
}
