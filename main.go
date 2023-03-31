// SeascapeSDS comes both with SDK and Core features.
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
	"github.com/blocklords/sds/security/vault"
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
		panic(err)
	}

	app_config, err := configuration.NewAppConfig(logger)
	if err != nil {
		logger.Fatal("configuration.NewAppConfig", "error", err)
	}

	// Prepare the parameters of database
	app_config.SetDefaults(db.DatabaseConfigurations)
	database_parameters, err := db.GetParameters(app_config)
	if err != nil {
		logger.Fatal("db.GetParameters", "error", err)
	}
	database_credentials := db.GetDefaultCredentials(app_config)

	// Prepare the security layer if plain wasn't given
	logger.Info("Setting up Vault connection and authentication layer...")
	var vault_database *vault.DatabaseVault
	if !app_config.Plain {
		app_config.SetDefaults(vault.VaultConfigurations)
		v, err := vault.New(logger, app_config)
		if err != nil {
			logger.Fatal("vault error", "message", err)
		}

		go v.PeriodicallyRenewLeases()
		go v.RunController()

		// database credentials from the vault
		app_config.SetDefaults(vault.DatabaseVaultConfigurations)
		vault_database, _ := vault.NewDatabase(v)
		database_credentials, err = vault_database.GetDatabaseCredentials()
		if err != nil {
			logger.Fatal("reading database credentials from vault: %v", err)
		}

		// Setup the Security layer. Any outside services that wants to connect
		// All incoming messages are encrypted and authenticated.
		if err := security.New(app_config.DebugSecurity).StartAuthentication(); err != nil {
			logger.Fatal("security: %v", err)
		}
	}

	// Set the database connection
	database, err := db.Open(logger, database_parameters, database_credentials)
	if err != nil {
		logger.Fatal("database error", "message", err)
	}
	if !app_config.Plain {
		go vault_database.PeriodicallyRenewLeases(database.Reconnect)
	}

	defer func() {
		_ = database.Close()
	}()

	/////////////////////////////////////////////////////////////////////////
	//
	// Run the Core services:
	//
	/////////////////////////////////////////////////////////////////////////
	var core_service *service.Service
	if app_config.Plain {
		core_service, err = service.NewExternal(service.CORE, service.THIS, app_config)
		if err != nil {
			logger.Fatal("external core service error", "message", err)
		}
	} else {
		core_service, err = service.NewSecure(service.CORE, service.THIS, app_config)
		if err != nil {
			logger.Fatal("external core service error", "message", err)
		}
	}

	// Prepare the external message receiver
	// This is aimed to be connected by SDS Gateway
	router, err := controller.NewRouter(core_service, logger)
	if err != nil {
		logger.Fatal("controller new router", "error", err)
	}

	// Prepare the list of core services that
	// The router will redirect the data to the services
	err = router.AddDealers(static.Service(), categorizer.Service(), blockchain.Service())
	if err != nil {
		logger.Fatal("router.AddDealers", "message", err)
	}

	// Start the core services
	go static.Run(app_config, database)
	go categorizer.Run(app_config, database)
	go blockchain.Run(app_config)

	// Start the external services
	router.Run()
}
