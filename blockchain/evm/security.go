package evm

import (
	"fmt"

	"github.com/blocklords/gosds/app/service"
)

// Return the whitelisted services that can access to this service
func get_whitelisted_services() ([]*service.Service, error) {
	services := make([]*service.Service, 2)

	if s, err := service.New(service.GATEWAY, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.GATEWAY, err)
	} else {
		services[0] = s
	}

	if s, err := service.New(service.CATEGORIZER, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.CATEGORIZER, err)
	} else {
		services[1] = s
	}

	return services, nil
}

// The services that can subscribe to the broadcaster
func get_whitelisted_subscribers() ([]*service.Service, error) {
	services := make([]*service.Service, 1)

	if s, err := service.New(service.CATEGORIZER, service.SUBSCRIBE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.CATEGORIZER, err)
	} else {
		services[0] = s
	}

	return services, nil
}
