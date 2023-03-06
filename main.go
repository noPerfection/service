// SeascapeSDS comes both with SDK and Core features.
package main

import (
	"fmt"

	"github.com/blocklords/sds/app/log"

	"sync"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/blockchain"
	"github.com/blocklords/sds/categorizer"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/security"
	"github.com/blocklords/sds/security/vault"
	"github.com/blocklords/sds/static"
)

/** SeascapeSDS + its SDK to use it.*/
func main() {
	logger := log.New()

	logger.SetPrefix("main")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	app_config, err := configuration.NewAppConfig(logger)
	if err != nil {
		new_err := fmt.Errorf("configuration: %v", err)
		logger.Fatal(new_err)
	}

	app_config.SetDefaults(db.DatabaseConfigurations)
	database_parameters, err := db.GetParameters(app_config)
	if err != nil {
		logger.Fatal("database parameter fetching: %v", err)
		panic(1)
	}
	database_credetnails := db.GetDefaultCredentials(app_config)

	var v *vault.Vault = nil
	var database *db.Database = nil

	if !app_config.Plain {
		logger.Info("Setting up Vault connection and authentication layer...")
		app_config.SetDefaults(vault.VaultConfigurations)

		new_vault, err := vault.New(logger, app_config)
		if err != nil {
			logger.Fatal("vault error", "message", err)
		} else {
			v = new_vault
		}

		// database credentials from the vault
		new_credentials, err := v.GetDatabaseCredentials()
		if err != nil {
			logger.Fatal("reading database credentials from vault: %v", err)
		} else {
			database_credetnails = new_credentials
		}
		go v.PeriodicallyRenewLeases(database.Reconnect)
		go v.RunController()

		s := security.New(app_config.DebugSecurity)
		if err := s.StartAuthentication(); err != nil {
			logger.Fatal("security: %v", err)
		}
	}

	database, err = db.Open(logger, database_parameters, database_credetnails)
	if err != nil {
		logger.Fatal("database error", "message", err)
	}
	defer func() {
		_ = database.Close()
	}()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		static.Run(app_config, database)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		categorizer.Run(app_config, database)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		blockchain.Run(app_config)
		wg.Done()
	}()
	wg.Wait()

	// fmt.Println("query the database")
	// result, err := database.Query(context.TODO(), "SELECT address FROM static_smartcontract WHERE 1", nil)
	// if err != nil {
	// log.Fatalf("test query to database: %v", err)
	// }

	// fmt.Println("database query result: ")
	// for _, address := range result {
	// fmt.Println("address ", address)
	// }
	logger.Info("Gracefully shutted down")
}
