package service

import (
	"strconv"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/common/data_type/key_value"
)

const localhost = "localhost"
const intial_port = 4000

// Returns the default configuration for the service
func DefaultConfiguration(service_type ServiceType) configuration.DefaultConfig {
	service_types := service_types()

	for i, value := range service_types {
		if service_type != value {
			continue
		}
		name := service_type.ToString()

		port_value := intial_port + (i * 10) + 1
		broadcast_port_value := intial_port + (i * 10) + 2

		// names
		parameters := map[string]interface{}{
			name + "_HOST": localhost,
			name + "_PORT": strconv.Itoa(port_value),

			name + "_BROADCAST_HOST": localhost,
			name + "_BROADCAST_PORT": strconv.Itoa(broadcast_port_value),
		}

		default_config := configuration.DefaultConfig{
			Title:      service_type.ToString(),
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
		name := service_type.ToString()

		port_value := intial_port + (i * 10) + 1
		broadcast_port_value := intial_port + (i * 10) + 2

		// names
		parameters := map[string]interface{}{
			name + "_HOST": localhost,
			name + "_PORT": strconv.Itoa(port_value),

			name + "_BROADCAST_HOST": localhost,
			name + "_BROADCAST_PORT": strconv.Itoa(broadcast_port_value),
		}

		default_config := configuration.DefaultConfig{
			Title:      service_type.ToString(),
			Parameters: key_value.New(parameters),
		}

		default_configs[i] = default_config
	}

	return default_configs
}
