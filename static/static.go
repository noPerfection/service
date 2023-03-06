package static

import (
	"github.com/blocklords/sds/static/handler"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/account"
	app_log "github.com/blocklords/sds/app/log"

	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/db"
)

func Run(app_config *configuration.Config, db_connection *db.Database) {
	var commands = controller.CommandHandlers{
		"abi_get":                      handler.AbiGet,
		"abi_get_by_smartcontract_key": handler.AbiGetBySmartcontractKey,
		"abi_register":                 handler.AbiRegister,

		"smartcontract_get":        handler.SmartcontractGet,
		"smartcontract_register":   handler.SmartcontractRegister,
		"smartcontract_filter":     handler.SmartcontractFilter,
		"smartcontract_key_filter": handler.SmartcontractKeyFilter,

		"configuration_get":      handler.ConfigurationGet,
		"configuration_register": handler.ConfigurationRegister,

		"network_id_get_all": handler.NetworkGetIds,
		"network_get_all":    handler.NetworkGetAll,
		"network_get":        handler.NetworkGet,
	}

	logger := app_log.New()
	logger.SetPrefix("static")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	// Getting the services which has access to the SDS Static
	static_env, err := service.New(service.STATIC, service.THIS)
	if err != nil {
		logger.Fatal("service configuration", "message", err)
	}

	// we whitelist before we initiate the reply controller
	if !app_config.Plain {
		logger.Info("getting whitelisted services")
		whitelisted_services, err := get_whitelisted_services()
		if err != nil {
			logger.Fatal("whitelist service", "message", err)
		}
		accounts := account.NewServices(whitelisted_services)
		controller.AddWhitelistedAccounts(static_env, accounts)
		logger.Info("add whitelisted users", "whitelisted", accounts.Names())
	}

	reply, err := controller.NewReply(static_env)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	} else {
		reply.SetLogger(logger)
	}

	if !app_config.Plain {
		logger.Info("set the private key")
		err := reply.SetControllerPrivateKey()
		if err != nil {
			logger.Fatal("controller security", "message", err)
		}
	}

	err = reply.Run(db_connection, commands)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	}
}
