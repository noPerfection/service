// Package storage defines the service
// that handles the data processing and storing in the database.
//
// The storage works with the three kind of data:
//   - abi of the smartcontract
//   - smartcontract is the smartcontract linked to the abi.
//   - configuration is the Topic linked to the smartcontract.
package storage

import (
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/parameter"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/storage/abi"
	storage_conf "github.com/blocklords/sds/storage/configuration"
	"github.com/blocklords/sds/storage/handler"
	"github.com/blocklords/sds/storage/smartcontract"
)

// Return the list of command handlers for this service
var CommandHandlers = handler.CommandHandlers()

// Returns this service's configuration
// Returns nil if the service parameters doesn't exist in the app/service.service_types
func Service() *parameter.Service {
	service, _ := parameter.Inprocess(parameter.STORAGE)
	return service
}

// Start the SDS Storage core service.
// It keeps the storage data:
// - smartcontract abi
// - smartcontract information
// - configuration (a relationship between common/topic.Topic and storage.Smartcontract).
func Run(app_config *configuration.Config) {
	logger, _ := log.New("storage", log.WITH_TIMESTAMP)

	// Getting the services which has access to the SDS Storage
	storage_env := Service()
	database_service, err := parameter.Inprocess(parameter.DATABASE)
	if err != nil {
		logger.Fatal("service.Inprocess(service.DATABASE)", "error", err)
	}

	db_socket, err := remote.InprocRequestSocket(database_service.Url(), logger, app_config)
	if err != nil {
		logger.Fatal("remote.InprocRequestSocket", "error", err)
	}

	reply, err := controller.NewReply(storage_env, logger)
	if err != nil {
		logger.Fatal("reply controller", "message", err)
	}

	// the global parameters to reduce
	// database queries
	var crud database.Crud = &abi.Abi{}
	var abis []*abi.Abi

	err = crud.SelectAll(db_socket, &abis)
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

	// storage smartcontracts
	crud = &smartcontract.Smartcontract{}
	var smartcontracts []*smartcontract.Smartcontract

	err = crud.SelectAll(db_socket, &smartcontracts)
	if err != nil {
		logger.Fatal("smartcontract.SelectAll", "error", err)
	}
	smartcontracts_list := key_value.NewList()
	for _, sm := range smartcontracts {
		err := smartcontracts_list.Add(sm.SmartcontractKey, sm)
		if err != nil {
			logger.Fatal("smartcontracts_list.Add", "error", err)
		}
	}

	// storage configurations
	crud = &storage_conf.Configuration{}
	var configurations []*storage_conf.Configuration

	err = crud.SelectAll(db_socket, &configurations)
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
