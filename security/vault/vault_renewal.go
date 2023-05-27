package vault

import (
	"context"
	"fmt"

	"github.com/blocklords/sds/db/handler"
	"github.com/blocklords/sds/service/parameter"
	remote_parameter "github.com/blocklords/sds/service/remote/parameter"
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
func (v *Vault) periodically_renew_leases() {
	/* */ v.logger.Info("renew / recreate secrets loop: begin")
	defer v.logger.Info("renew / recreate secrets loop: end")

	for {
		ctx, cancel_func := remote_parameter.NewContextWithTimeout(context.TODO(), v.app_config)
		renewed, err := v.renewLeases(ctx, v.auth_token)
		cancel_func()
		if err != nil {
			v.logger.Fatal("renew error", "error", err) // simplified error handling
		}

		if renewed&exitRequested != 0 {
			return
		}

		if renewed&expiringAuthToken != 0 {
			v.logger.Info("auth token: can no longer be renewed; will log in again")

			ctx, cancel_func := remote_parameter.NewContextWithTimeout(context.TODO(), v.app_config)
			auth_token, err := v.login(ctx)
			cancel_func()
			if err != nil {
				v.logger.Fatal("login authentication error", "error", err) // simplified error handling
			}

			v.auth_token = auth_token
		}
	}
}

func (v *Vault) periodically_renew_database_leases() {
	/* */ v.logger.Info("renew / recreate secrets loop: begin")
	defer v.logger.Info("renew / recreate secrets loop: end")

	database_client, err := handler.PushSocket()
	if err != nil {
		v.logger.Fatal("remote.InprocRequestSocket.Inproc", "service type", parameter.DATABASE, "error", err)
	}

	for {
		ctx, cancel_func := remote_parameter.NewContextWithTimeout(context.TODO(), v.app_config)
		renewed, err := v.renewLeases(ctx, v.database_vault.database_auth_token)
		cancel_func()
		if err != nil {
			v.logger.Fatal("renew error", "error", err) // simplified error handling
		}

		if renewed&exitRequested != 0 {
			return
		}

		if renewed&expiringDatabaseCredentialsLease != 0 {
			v.logger.Fatal("database credentials: can no longer be renewed; will fetch new credentials & reconnect")

			databaseCredentials, err := v.get_db_credentials()
			if err != nil {
				v.logger.Fatal("database credentials error", "error", err) // simplified error handling
			}

			if err := handler.NEW_CREDENTIALS.Push(database_client, databaseCredentials); err != nil {
				v.logger.Fatal("database connection error", "error", err) // simplified error handling
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
func (v *Vault) renewLeases(ctx context.Context, authToken *hashicorp.Secret) (renewResult, error) {
	/* */ v.logger.Info("renew cycle: begin")
	defer v.logger.Info("renew cycle: end")

	var authTokenWatcher *hashicorp.LifetimeWatcher

	// auth token
	authTokenWatcher, err := v.client.NewLifetimeWatcher(&hashicorp.LifetimeWatcherInput{
		Secret: authToken,
	})
	if err != nil {
		return renewError, fmt.Errorf("unable to initialize auth token lifetime watcher: %w", err)
	}

	go authTokenWatcher.Start()
	defer authTokenWatcher.Stop()

	// monitor events from both watchers
	for {
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

		case info := <-authTokenWatcher.RenewCh():
			v.logger.Info("auth token: successfully renewed", "remaining duration", info.Secret.Auth.LeaseDuration)
		}
	}
}

func (v *DatabaseVault) renewLeases(ctx context.Context, databaseCredentialsLease *hashicorp.Secret) (renewResult, error) {
	/* */ v.logger.Info("renew cycle: begin")
	defer v.logger.Info("renew cycle: end")

	// database credentials
	databaseCredentialsWatcher, err := v.vault.client.NewLifetimeWatcher(&hashicorp.LifetimeWatcherInput{
		Secret: databaseCredentialsLease,
	})
	if err != nil {
		return renewError, fmt.Errorf("unable to initialize database credentials lifetime watcher: %w", err)
	}

	go databaseCredentialsWatcher.Start()
	defer databaseCredentialsWatcher.Stop()

	// monitor events from both watchers
	for {
		select {
		case <-ctx.Done():
			return exitRequested, nil
		case err := <-databaseCredentialsWatcher.DoneCh():
			return expiringDatabaseCredentialsLease, err
		case info := <-databaseCredentialsWatcher.RenewCh():
			v.logger.Info("database credentials: successfully renewed;", "remaining lease duration", info.Secret.LeaseDuration)
		}
	}
}
