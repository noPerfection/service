package static

import (
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/handler"
)

// Start the SDS Static core service.
// It keeps the static data:
// - smartcontract abi
// - smartcontract information
// - configuration (a relationship between common/topic.Topic and static.Smartcontract).
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
	}


// Returns this service's configuration
func Service() *service.Service {
	return service.Inprocess(service.STATIC)
}
	logger := log.New()
	logger.SetPrefix("static")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	// Getting the services which has access to the SDS Static
	static_env := service.Inprocess(service.STATIC)

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
