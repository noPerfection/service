// SDS Core is the group of
//   - core services,
//   - security layers
//   - db (Database)
//   - and finally an sdk to interact with SDS.
//
// Core services are:
//   - Static to keep the smartcontracts, their abi and topic
//   - Blockchain to connect to the remote blockchain nodes in a smart way
//   - Categorizer to decode the event logs and make sure users can interact with them over SDK.
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
	"github.com/blocklords/sds/blockchain"
	"github.com/blocklords/sds/categorizer"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/security"
	"github.com/blocklords/sds/static"
)

// SDS Core
//
// Router with security enabled.
// Router is connected from the Developer Gateway and Smartcontract Developer Gateway.
//
// Router has the request.
// Request could go to static
// Request could go to categorizer
// Request could go to blockchain
//
// Each of the services has the reply controller.
// The reply controller is replies back to the router.
//
// The router returns replies the result back to the user.
func main() {
	logger, err := log.New("sds-core", log.WITH_TIMESTAMP)
	if err != nil {
		logger.Fatal("log.New(`sds-core`)", "error", err)
	}

	app_config, err := configuration.NewAppConfig(logger)
	if err != nil {
		logger.Fatal("configuration.NewAppConfig", "error", err)
	}

	if app_config.Secure {
		security_service, err := security.New(app_config, logger)
		if err != nil {
			logger.Fatal("security.New", "error", err)
		}
		go security_service.Run()
		go db.Run(app_config, logger)
	}

	/////////////////////////////////////////////////////////////////////////
	//
	// Run the Core services:
	//
	/////////////////////////////////////////////////////////////////////////
	var core_service *service.Service
	core_service, err = service.NewExternal(service.CORE, service.THIS, app_config)
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
	err = router.AddDealers(static.Service(), categorizer.Service(), blockchain.Service())
	if err != nil {
		logger.Fatal("router.AddDealers", "message", err)
	}

	// Start the core services
	go static.Run(app_config)
	go categorizer.Run(app_config)
	go blockchain.Run(app_config)

	// Start the external services
	router.Run()
}
