package static

import (
	"github.com/blocklords/sds/static/handler"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/service"

	app_log "github.com/blocklords/sds/app/log"

	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/db"
)

func Run(app_config *configuration.Config, db_connection *db.Database) {
	var commands = controller.CommandHandlers{
		"abi_get": handler.AbiGetBySmartcontractKey,
		"abi_set": handler.AbiRegister,

		"smartcontract_get":        handler.SmartcontractGet,
		"smartcontract_set":        handler.SmartcontractRegister,
		"smartcontract_filter":     handler.SmartcontractFilter,
		"smartcontract_key_filter": handler.SmartcontractKeyFilter,

		"configuration_get": handler.ConfigurationGet,
		"configuration_set": handler.ConfigurationRegister,

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
	static_env, err := service.Inprocess(service.STATIC)
	if err != nil {
		logger.Fatal("service configuration", "message", err)
	}

	reply, err := controller.NewReply(static_env)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	} else {
		reply.SetLogger(logger)
	}

	err = reply.Run(commands, db_connection)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	}
}
