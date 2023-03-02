package blockchain

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/broadcast"
	"github.com/blocklords/gosds/app/controller"
	"github.com/blocklords/gosds/app/service"
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

	whitelisted_subscribers, err := get_whitelisted_subscribers()
	if err != nil {
		logger.Fatal("get_whitelisted_subscribers", "message", err)
	}
	subsribers := account.NewServices(whitelisted_subscribers)

	broadcast.AddWhitelistedAccounts(spaghetti_env, subsribers)
}

func set_curve_key(logger log.Logger, reply *controller.Controller, broadcaster *broadcast.Broadcast) {
	logger.Info("set the private keys")

	err := reply.SetControllerPrivateKey()
	if err != nil {
		logger.Fatal("controller.SetControllerPrivateKey", "message", err)
	}

	err = broadcaster.SetPrivateKey()
	if err != nil {
		logger.Fatal("broadcast.SetPrivateKey", "message", err)
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
