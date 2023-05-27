// SDS Core is the group of
//   - core services,
//   - security layers
//   - db (Database)
//   - and finally an sdk to interact with SDS.
//
// Core services are:
//   - Storage to keep the smartcontracts, their abi and topic
//   - Blockchain to connect to the remote blockchain nodes in a smart way
//   - Indexer to decode the event logs and make sure users can interact with them over SDK.
//
// For detailed documentation visit:
// https://github.com/blocklords/sds
//
// The security layers include two parts:
//   - credentials to enable authentication in the sockets
//   - vault to interact with the remote vault
//
// The database engine that SDS is using is Mysql.
package main

import (
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/configuration/argument"
	"github.com/blocklords/sds/blockchain"
	"github.com/blocklords/sds/db"
	indexer "github.com/blocklords/sds/indexer"
	"github.com/blocklords/sds/security"
	"github.com/blocklords/sds/storage"
)

// todo remove db from vault or make sure vault works if
// db is working too.
//
// # SDS Core
//
// Router with security enabled.
// Router is connected from the Developer Gateway and Smartcontract Developer Gateway.
//
// Router has the request.
// Request could go to storage
// Request could go to indexer
// Request could go to blockchain
//
// Each of the services has the reply controller.
// The reply controller is replies back to the router.
//
// The router returns replies the result back to the user.
func main() {
	logger, err := log.New("main", log.WITH_TIMESTAMP)
	if err != nil {
		logger.Fatal("log.New(`main`)", "error", err)
	}

	logger.Info("Load app configuration")
	app_config, err := configuration.NewAppConfig(logger)
	if err != nil {
		logger.Fatal("configuration.NewAppConfig", "error", err)
	}
	logger.Info("App configuration loaded successfully")

	if app_config.Secure {
		logger.Info("Security enabled, start security service")
		security_service, err := security.New(app_config, logger)
		if err != nil {
			logger.Fatal("security.New", "error", err)
		}
		go security_service.Run()
	} else {
		logger.Warn("App is running in an unsafe environment")
	}

	var dealers []*service.Service
	run_db := true

	// Core sds could come up with one service only.
	// That service is included as --service=<>
	if argument.Exist(argument.SERVICE) {
		only_service, err := argument.GetValue(argument.SERVICE)
		if err != nil {
			logger.Fatal("argument.GetValue", "name", argument.SERVICE, "error", err)
		}
		service_type, err := service.NewServiceType(only_service)
		if err != nil {
			logger.Fatal("service.NewServiceType", "name", only_service, "error", err)
		}

		if service_type == service.STORAGE {
			dealers = []*service.Service{storage.Service()}
		} else if service_type == service.INDEXER {
			dealers = []*service.Service{indexer.Service()}
		} else if service_type == service.BLOCKCHAIN {
			dealers = []*service.Service{blockchain.Service()}
			run_db = false
		} else {
			logger.Fatal("Unsupported service", "service_type", service_type)
		}
	} else {
		dealers = []*service.Service{storage.Service(), indexer.Service(), blockchain.Service()}
	}

	if run_db {
		logger.Info("Run the database service")
		go db.Run(app_config)
	}

	/////////////////////////////////////////////////////////////////////////
	//
	// Run the Core services:
	//
	/////////////////////////////////////////////////////////////////////////

	logger.Info("Get CORE service parameters")

	core_service, err := service.NewExternal(service.CORE, service.THIS, app_config)
	if err != nil {
		logger.Fatal("external core service error", "message", err)
	}

	// Prepare the external message receiver
	// This is aimed to be connected by SDS Gateway
	router, err := controller.NewRouter(core_service, logger)
	if err != nil {
		logger.Fatal("controller new router", "error", err)
	}

	// todo add SetSecurity for router
	// if app_config.Secure {
	// 	creds, err := service_credentials.ServiceCredentials(service.CORE, service.THIS, app_config)
	// 	if err != nil {
	// 		logger.Fatal("controller new router", "error", err)
	// 	}
	// 	router.SetCreds(creds)
	// }

	// Prepare the list of core services that
	// The router will redirect the data to the services
	err = router.AddDealers(dealers...)
	if err != nil {
		logger.Fatal("router.AddDealers", "message", err)
	}

	// Start the core services
	go storage.Run(app_config)
	go indexer.Run(app_config)
	go blockchain.Run(app_config)

	// Start the external services
	router.Run()
}
