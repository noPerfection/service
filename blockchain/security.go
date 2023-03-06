package blockchain

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/account"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/service"
)

func whitelist_access(logger log.Logger, spaghetti_env *service.Service) {
	logger.Info("get the whitelisted services")

	whitelisted_services, err := get_whitelisted_services()
	if err != nil {
		logger.Fatal("get_whitelisted_services", "message", err)
	}
	accounts := account.NewServices(whitelisted_services)
	controller.AddWhitelistedAccounts(spaghetti_env, accounts)

	logger.Info("get the whitelisted subscribers")
}

func set_curve_key(logger log.Logger, reply *controller.Controller) {
	logger.Info("set the private keys")

	err := reply.SetControllerPrivateKey()
	if err != nil {
		logger.Fatal("controller.SetControllerPrivateKey", "message", err)
	}
}

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
