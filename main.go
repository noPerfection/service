// SeascapeSDS comes both with SDK and Core features.
package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/categorizer"
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/security/vault"
	"github.com/blocklords/gosds/spaghetti"
	"github.com/blocklords/gosds/static"
)

/** SeascapeSDS + its SDK to use it.*/
func main() {
	fmt.Println("SeascapeSDS!!!")

	app_config, err := configuration.NewAppConfig()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	if app_config.Plain {
		fmt.Println("Security is switched off")
	} else {
		fmt.Println("Security is enabled. add '--plain' to switch off security")
	}

	// Initiate the default service configurations
	service_default_configs := service.DefaultConfigurations()
	for _, service_default_config := range service_default_configs {
		app_config.SetDefaults(service_default_config)
	}

	app_config.SetDefaults(db.DatabaseConfigurations)
	database_parameters, err := db.GetParameters(app_config)
	if err != nil {
		log.Fatalf("database parameter fetching: %v", err)
	}
	database_credetnails := db.GetDefaultCredentials(app_config)

	var v *vault.Vault = nil
	var database *db.Database = nil

	if !app_config.Plain {
		app_config.SetDefaults(vault.VaultConfigurations)

		new_vault, auth_token, err := vault.New(app_config)
		if err != nil {
			log.Fatalf("vault initiation: %v", err)
		} else {
			v = new_vault
		}

		// database
		new_credentials, databaseCredentialsLease, err := v.GetDatabaseCredentials()
		if err != nil {
			log.Fatalf("reading database credentials from vault: %v", err)
		} else {
			database_credetnails = new_credentials
		}

		// start the lease-renewal goroutine & wait for it to finish on exit
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			v.PeriodicallyRenewLeases(auth_token, databaseCredentialsLease, database.Reconnect)
			wg.Done()
		}()
		defer func() {
			wg.Wait()
		}()
	}

	database, err = db.Open(database_parameters, database_credetnails)
	if err != nil {
		log.Fatalf("database connection: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		static.Run(app_config, database, v)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		categorizer.Run(app_config, database, v)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		spaghetti.Run(app_config, database, v)
		wg.Done()
	}()
	defer func() {
		wg.Wait()
	}()

	fmt.Println("query the database")
	result, err := database.Query(context.TODO(), "SELECT address FROM static_smartcontract WHERE 1", nil)
	if err != nil {
		log.Fatalf("test query to database: %v", err)
	}

	fmt.Println("database query result: ")
	for _, address := range result {
		fmt.Println("address ", address)
	}
}
