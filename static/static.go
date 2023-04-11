// Package static defines the service
// that handles the data processing and storing in the database.
//
// The static works with the three kind of data:
//   - abi of the smartcontract
//   - smartcontract is the smartcontract linked to the abi.
//   - configuration is the Topic linked to the smartcontract.
package static

import (
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/static/abi"
	static_conf "github.com/blocklords/sds/static/configuration"
	"github.com/blocklords/sds/static/handler"
	"github.com/blocklords/sds/static/smartcontract"
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
func Run(app_config *configuration.Config) {
	logger, _ := log.New("static", log.WITH_TIMESTAMP)

	// Getting the services which has access to the SDS Static
	static_env := Service()
	database_service, err := service.Inprocess(service.DATABASE)
	if err != nil {
		logger.Fatal("service.Inprocess(service.DATABASE)", "error", err)
	}

	db_socket, err := remote.InprocRequestSocket(database_service.Url(), logger, app_config)
	if err != nil {
		logger.Fatal("remote.InprocRequestSocket", "error", err)
	}

	reply, err := controller.NewReply(static_env, logger)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	}

	// the global parameters to reduce
	// database queries
	abis, err := abi.GetAllFromDatabase(db_socket)
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

	// static smartcontracts
	smartcontracts, err := smartcontract.GetAllFromDatabase(db_socket)
	if err != nil {
		logger.Fatal("smartcontract.GetAllFromDatabase: %w", err)
	}
	smartcontracts_list := key_value.NewList()
	for _, sm := range smartcontracts {
		err := smartcontracts_list.Add(sm.SmartcontractKey, sm)
		if err != nil {
			logger.Fatal("smartcontracts_list.Add: %w", err)
		}
	}

	// static configurations
	configurations, err := static_conf.GetAllFromDatabase(db_socket)
	if err != nil {
		logger.Fatal("configuration.GetAllFromDatabase: %w", err)
	}
	configurations_list := key_value.NewList()
	for _, conf := range configurations {
		err := configurations_list.Add(conf.Topic, conf)
		if err != nil {
			logger.Fatal("configurations_list.Add: %w", err)
		}
	}

	err = reply.Run(
		CommandHandlers,
		db_socket,
		abi_list,
		smartcontracts_list,
		configurations_list,
	)
	if err != nil {
		logger.Fatal("reply controller", "error", err)
	}
}
