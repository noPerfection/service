package vault

import (
	"context"
	"fmt"
	"log"

	"github.com/blocklords/gosds/app/env"
	"github.com/blocklords/gosds/db"
	hashicorp "github.com/hashicorp/vault/api"
)

// Once you've set the token for your Vault client, you will need to
// periodically renew it. Likewise, the database credentials lease will expire
// at some point and also needs to be renewed periodically.
//
// A function like this one should be run as a goroutine to avoid blocking.
// Production applications may also need to be more tolerant of failures and
// retry on errors rather than exiting.
//
// Additionally, enterprise Vault users should be aware that due to eventual
// consistency, the API may return unexpected errors when running Vault with
// performance standbys or performance replication, despite the client having
// a freshly renewed token. See the link below for several ways to mitigate
// this which are outside the scope of this code sample.
//
// ref: https://www.vaultproject.io/docs/enterprise/consistency#vault-1-7-mitigations
func (v *Vault) PeriodicallyRenewLeases(db_reconnect func(ctx context.Context, credentials db.DatabaseCredentials) error) {
	/* */ log.Println("renew / recreate secrets loop: begin")
	defer log.Println("renew / recreate secrets loop: end")

	for {
		renewed, err := v.renewLeases(v.context, v.auth_token, v.database_auth_token)
		if err != nil {
			log.Fatalf("renew error: %v", err) // simplified error handling
		}

		if renewed&exitRequested != 0 {
			return
		}

		if renewed&expiringAuthToken != 0 {
			log.Printf("auth token: can no longer be renewed; will log in again")

			auth_token, err := v.login(v.context)
			if err != nil {
				log.Fatalf("login authentication error: %v", err) // simplified error handling
			}

			v.auth_token = auth_token
		}

		if renewed&expiringDatabaseCredentialsLease != 0 {
			log.Printf("database credentials: can no longer be renewed; will fetch new credentials & reconnect")

			databaseCredentials, err := v.GetDatabaseCredentials()
			if err != nil {
				log.Fatalf("database credentials error: %v", err) // simplified error handling
			}

			if err := db_reconnect(v.context, databaseCredentials); err != nil {
				log.Fatalf("database connection error: %v", err) // simplified error handling
			}
		}
	}
}

// renewResult is a bitmask which could contain one or more of the values below
type renewResult uint8

const (
	renewError renewResult = 1 << iota
	exitRequested
	expiringAuthToken                // will be revoked soon
	expiringDatabaseCredentialsLease // will be revoked soon
)

// renewLeases is a blocking helper function that uses LifetimeWatcher
// instances to periodically renew the given secrets when they are close to
// their 'token_ttl' expiration times until one of the secrets is close to its
// 'token_max_ttl' lease expiration time.
func (v *Vault) renewLeases(ctx context.Context, authToken, databaseCredentialsLease *hashicorp.Secret) (renewResult, error) {
	/* */ log.Println("renew cycle: begin")
	defer log.Println("renew cycle: end")

	var authTokenWatcher *hashicorp.LifetimeWatcher

	// auth token
	secure := env.GetString("SDS_VAULT_SECURE")
	if secure == "true" {
		authTokenWatcher, err := v.client.NewLifetimeWatcher(&hashicorp.LifetimeWatcherInput{
			Secret: authToken,
		})
		if err != nil {
			return renewError, fmt.Errorf("unable to initialize auth token lifetime watcher: %w", err)
		}

		go authTokenWatcher.Start()
		defer authTokenWatcher.Stop()
	}

	// database credentials
	databaseCredentialsWatcher, err := v.client.NewLifetimeWatcher(&hashicorp.LifetimeWatcherInput{
		Secret: databaseCredentialsLease,
	})
	if err != nil {
		return renewError, fmt.Errorf("unable to initialize database credentials lifetime watcher: %w", err)
	}

	go databaseCredentialsWatcher.Start()
	defer databaseCredentialsWatcher.Stop()

	// monitor events from both watchers
	for {
		if secure == "true" {
			select {
			case <-ctx.Done():
				return exitRequested, nil

				// DoneCh will return if renewal fails, or if the remaining lease
				// duration is under a built-in threshold and either renewing is not
				// extending it or renewing is disabled.  In both cases, the caller
				// should attempt a re-read of the secret. Clients should check the
				// return value of the channel to see if renewal was successful.
			case err := <-authTokenWatcher.DoneCh():
				// Leases created by a token get revoked when the token is revoked.
				return expiringAuthToken | expiringDatabaseCredentialsLease, err

			case err := <-databaseCredentialsWatcher.DoneCh():
				return expiringDatabaseCredentialsLease, err

			// RenewCh is a channel that receives a message when a successful
			// renewal takes place and includes metadata about the renewal.
			case info := <-authTokenWatcher.RenewCh():
				log.Printf("auth token: successfully renewed; remaining duration: %ds", info.Secret.Auth.LeaseDuration)

			case info := <-databaseCredentialsWatcher.RenewCh():
				log.Printf("database credentials: successfully renewed; remaining lease duration: %ds", info.Secret.LeaseDuration)
			}
		} else {
			select {
			case <-ctx.Done():
				return exitRequested, nil
			case err := <-databaseCredentialsWatcher.DoneCh():
				return expiringDatabaseCredentialsLease, err
			case info := <-databaseCredentialsWatcher.RenewCh():
				log.Printf("database credentials: successfully renewed; remaining lease duration: %ds", info.Secret.LeaseDuration)
			}
		}
	}
}
