package service

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/log"
)

// New loads the logger and loads the configuration
func New(prefix string) (*log.Logger, *configuration.Config, error) {
	logger, err := log.New(prefix, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a logger with prefix %s: %w", prefix, err)
	}

	appConfig, err := configuration.NewAppConfig(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load the app configuration: %w", err)
	}

	if len(appConfig.Services) == 0 {
		return nil, nil, fmt.Errorf("services is empty: %w", err)
	}

	return &logger, appConfig, nil
}
