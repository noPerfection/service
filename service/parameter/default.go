// Package service defines the SDS part and its parameters to identify it on
// and connect to it.
// For example, the parameters includes the host name and port if you want to connect to it
// via TCP protocol.
package parameter

import (
	"strconv"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/configuration"
)

const (
	localhost   = "localhost"
	intial_port = 4000
	port_offset = 10
)

// i is the service index
func calculate_port(i int) int {
	return intial_port + (i * port_offset)
}

// Returns the default configuration for the service
//
// The first service's launched in the initial_port.
// At most the service should have 10 available ports.
//
// Each service's port number is incremented by 10.
func DefaultConfiguration(service_type ServiceType) configuration.DefaultConfig {
	service_types := service_types()

	for i, value := range service_types {
		if service_type != value {
			continue
		}
		name := service_type.ToString()

		port_value := calculate_port(i)
		broadcast_port_value := port_value + 1

		// names
		parameters := map[string]interface{}{
			name + "_HOST": localhost,
			name + "_PORT": strconv.Itoa(port_value),

			name + "_BROADCAST_HOST": localhost,
			name + "_BROADCAST_PORT": strconv.Itoa(broadcast_port_value),
		}

		default_config := configuration.DefaultConfig{
			Title:      "SERVICE " + service_type.ToString(),
			Parameters: key_value.New(parameters),
		}

		return default_config
	}

	return configuration.DefaultConfig{Title: ""}
}

// Returns the list of default configurations for all services
func DefaultConfigurations() []configuration.DefaultConfig {
	service_types := service_types()
	default_configs := make([]configuration.DefaultConfig, len(service_types))

	for i, service_type := range service_types {
		default_config := DefaultConfiguration(service_type)
		default_configs[i] = default_config
	}

	return default_configs
}
