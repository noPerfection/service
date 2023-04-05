package static

import (
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"
	"github.com/blocklords/sds/static/handler"
)

// Return the list of command handlers for this service
var CommandHandlers = handler.CommandHandlers()

// Returns this service's configuration
// Returns nil if the service parameters doesn't exist in the app/service.service_types
func Service() *service.Service {
	service, _ := service.Inprocess(service.STATIC)
	return service
}

// Start the SDS Static core service.
// It keeps the static data:
// - smartcontract abi
// - smartcontract information
// - configuration (a relationship between common/topic.Topic and static.Smartcontract).
func Run(_ *configuration.Config, db_connection *db.Database) {
	logger, _ := log.New("static", log.WITH_TIMESTAMP)

	logger.Info("starting")

	// Getting the services which has access to the SDS Static
	static_env := Service()

	reply, err := controller.NewReply(static_env, logger)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	}

	// the global parameters to reduce
	// database queries
	abis, err := abi.GetAllFromDatabase(db_connection)
	if err != nil {
		logger.Fatal("abi.GetAllFromDatabase: %w", err)
	}
	abi_list := key_value.NewList()
	for _, abi := range abis {
		err := abi_list.Add(abi.Id, abi)
		if err != nil {
			logger.Fatal("abi_list.Add: %w", err)
		}
	}

	err = reply.Run(CommandHandlers, db_connection, abi_list)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	}
}
