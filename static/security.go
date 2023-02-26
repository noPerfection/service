package static

import (
	"fmt"

	"github.com/blocklords/gosds/app/service"
)

// Return the whitelisted services that can access to this service
func get_whitelisted_services() ([]*service.Service, error) {
	services := make([]*service.Service, 8)

	if s, err := service.New(service.DEVELOPER_GATEWAY, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.DEVELOPER_GATEWAY, err)
	} else {
		services[0] = s
	}

	if s, err := service.New(service.GATEWAY, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.GATEWAY, err)
	} else {
		services[1] = s
	}

	if s, err := service.New(service.CATEGORIZER, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.CATEGORIZER, err)
	} else {
		services[2] = s
	}

	if s, err := service.New(service.BUNDLE, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.BUNDLE, err)
	} else {
		services[3] = s
	}

	if s, err := service.New(service.LOG, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.LOG, err)
	} else {
		services[4] = s
	}

	if s, err := service.New(service.READER, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.READER, err)
	} else {
		services[5] = s
	}

	if s, err := service.New(service.WRITER, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.WRITER, err)
	} else {
		services[6] = s
	}

	if s, err := service.New(service.SPAGHETTI, service.REMOTE); err != nil {
		return nil, fmt.Errorf("failed to get '%s' service configuration: %v", service.SPAGHETTI, err)
	} else {
		services[7] = s
	}

	return services, nil
}
