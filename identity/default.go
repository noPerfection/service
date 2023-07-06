// Package identity defines the service identity.
// For example, the parameters includes the host name and port if you want to connect to it
// via TCP protocol.
package identity

import (
	"strconv"

	"github.com/Seascape-Foundation/sds-common-lib/data_type/key_value"
	"github.com/Seascape-Foundation/sds-service-lib/configuration"
)

const (
	localhost   = "localhost"
	initialPort = 4000
	portOffset  = 10
)

// is the service index
func calculatePort(i int) int {
	return initialPort + (i * portOffset)
}

// DefaultConfiguration Returns the default configuration for the service
//
// The first service's launched in the initial_port.
// At most the service should have 10 available ports.
//
// Each service's port number is incremented by 10.
func DefaultConfiguration(serviceType ServiceType) configuration.DefaultConfig {
	serviceTypes := serviceTypes()

	for i, value := range serviceTypes {
		if serviceType != value {
			continue
		}
		name := serviceType.ToString()

		portValue := calculatePort(i)
		broadcastPortValue := portValue + 1

		// names
		parameters := map[string]interface{}{
			name + "_HOST": localhost,
			name + "_PORT": strconv.Itoa(portValue),

			name + "_BROADCAST_HOST": localhost,
			name + "_BROADCAST_PORT": strconv.Itoa(broadcastPortValue),
		}

		defaultConfig := configuration.DefaultConfig{
			Title:      "SERVICE " + serviceType.ToString(),
			Parameters: key_value.New(parameters),
		}

		return defaultConfig
	}

	return configuration.DefaultConfig{Title: ""}
}

// DefaultConfigurations Returns the list of default configurations for all services
func DefaultConfigurations() []configuration.DefaultConfig {
	serviceTypes := serviceTypes()
	defaultConfigs := make([]configuration.DefaultConfig, len(serviceTypes))

	for i, serviceType := range serviceTypes {
		defaultConfig := DefaultConfiguration(serviceType)
		defaultConfigs[i] = defaultConfig
	}

	return defaultConfigs
}
