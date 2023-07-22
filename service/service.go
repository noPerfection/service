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
