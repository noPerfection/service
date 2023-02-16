// SeascapeSDS comes both with SDK and Core features.
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/env"
	"github.com/blocklords/gosds/static"
	"github.com/blocklords/gosds/vault"
)

/** SeascapeSDS + its SDK to use it.*/
func main() {
	fmt.Println("SeascapeSDS!!!")

	// load any environment files for auto configuration.
	err := env.LoadAnyEnv()
	if err != nil {
		panic(err)
	}

	v, auth_token, err := vault.New()
	if err != nil {
		panic(err)
	}

	// database
	databaseCredentials, databaseCredentialsLease, err := v.GetDatabaseCredentials()
	if err != nil {
		panic(err)
	}

	database, err := db.Open(databaseCredentials)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = database.Close()
	}()

	go static.Run(v, database)

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

	fmt.Println("query the database")
	result, err := database.Query(context.TODO(), "SELECT address FROM static_smartcontract WHERE 1", nil)
	if err != nil {
		panic(err)
	}

	fmt.Println("database query result: ")
	for _, address := range result {
		fmt.Println("address ", address)
	}
}
