package service

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/context/dev"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/proxy"
	"strings"
)

func PrepareContext(context *configuration.Context) error {
	// get the extensions
	err := dev.Prepare(context)
	if err != nil {
		return fmt.Errorf("failed to prepare the context: %w", err)
	}

	return nil
}

// PrepareProxyConfiguration links the proxy with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func PrepareProxyConfiguration(requiredProxy string, config *configuration.Config, logger *log.Logger) error {
	err := dev.PrepareProxyConfiguration(config.Context, requiredProxy, logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareProxyConfiguration on %s: %w", requiredProxy, err)
	}

	proxy, err := dev.ReadProxyConfiguration(config.Context, requiredProxy)
	if err != nil {
		return fmt.Errorf("dev.ReadProxyConfiguration: %w", err)
	}

	proxyConfiguration := config.Service.GetProxy(requiredProxy)
	if proxyConfiguration == nil {
		config.Service.SetProxy(proxy)
	} else {
		if strings.Compare(proxyConfiguration.Url, proxy.Url) != 0 {
			return fmt.Errorf("the proxy urls are not matching. in your configuration: %s, in the deps: %s", proxyConfiguration.Url, proxy.Url)
		}
		if proxyConfiguration.Port != proxy.Port {
			return fmt.Errorf("the proxy ports are not matching. in your configuration: %d, in the deps: %d", proxyConfiguration.Port, proxy.Port)
		}
	}

	return nil
}

// PreparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the configuration, it makes sure that dependency is linted.
func PreparePipelineConfiguration(config *configuration.Config, proxyUrl string, controllerName string, logger *log.Logger) error {
	//
	// lint the dependency proxy's destination to the independent independent's controller
	//--------------------------------------------------
	proxyConfig, err := dev.ReadServiceConfiguration(config.Context, proxyUrl)
	if err != nil {
		return fmt.Errorf("dev.ReadServiceConfiguration of '%s': %w", proxyUrl, err)
	}

	destinationConfig, err := proxyConfig.GetController(proxy.DestinationName)
	if err != nil {
		return fmt.Errorf("getting dependency proxy's destination configuration failed: %w", err)
	}

	controllerConfig, err := config.Service.GetController(controllerName)
	if err != nil {
		return fmt.Errorf("getting '%s' controller from independent configuration failed: %w", controllerName, err)
	}

	// somehow it will work with only one instance. but in the future maybe another instances as well.
	destinationInstanceConfig := destinationConfig.Instances[0]
	instanceConfig := controllerConfig.Instances[0]

	if destinationInstanceConfig.Port != instanceConfig.Port {
		logger.Info("the dependency proxy destination not match to the controller",
			"proxy url", proxyUrl,
			"destination port", destinationInstanceConfig.Port,
			"independent controller port", instanceConfig.Port)

		destinationInstanceConfig.Port = instanceConfig.Port
		destinationConfig.Instances[0] = destinationInstanceConfig
		proxyConfig.SetController(destinationConfig)

		logger.Info("linting dependency proxy's destination port", "new port", instanceConfig.Port)
		logger.Warn("todo", 1, "if dependency proxy is running, then it should be restarted")
		err := dev.WriteServiceConfiguration(config.Context, proxyUrl, proxyConfig)
		if err != nil {
			return fmt.Errorf("dev.WriteServiceConfiguration for '%s': %w", proxyUrl, err)
		}
	}

	return nil
}
