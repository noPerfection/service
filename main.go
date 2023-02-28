// SeascapeSDS comes both with SDK and Core features.
package main

import (
	"fmt"

	"github.com/blocklords/gosds/app/log"

	"sync"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/blockchain"
	"github.com/blocklords/gosds/categorizer"
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/security"
	"github.com/blocklords/gosds/security/vault"
	"github.com/blocklords/gosds/static"
)

/** SeascapeSDS + its SDK to use it.*/
func main() {
	logger := log.New()

	logger.SetPrefix("main")
	logger.SetReportCaller(true)

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
		app_config.SetDefaults(vault.VaultConfigurations)

		new_vault, err := vault.New(app_config)
		if err != nil {
			logger.Fatal("vault initiation: %v", err)
		} else {
			v = new_vault
		}

		// database credentials from the vault
		new_credentials, err := v.GetDatabaseCredentials()
		if err != nil {
			logger.Fatal("reading database credentials from vault: %v", err)
			panic(1)
		} else {
			database_credetnails = new_credentials
		}
		go v.PeriodicallyRenewLeases(database.Reconnect)
		go v.RunController()

		s := security.New(app_config.DebugSecurity)
		if err := s.StartAuthentication(); err != nil {
			logger.Fatal("security: %v", err)
			panic(1)
		}
	}

	database, err = db.Open(database_parameters, database_credetnails)
	if err != nil {
		logger.Fatal("database connection: %v", err)
		panic(1)
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
	defer func() {
		wg.Wait()
	}()

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
