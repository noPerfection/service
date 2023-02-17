// Keep the credentials in a vault
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/blocklords/gosds/app/env"
	"github.com/blocklords/gosds/db"
	hashicorp "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
)

type Vault struct {
	client  *hashicorp.Client
	context context.Context
	path    string // Key-Value credentials

	// connection parameters
	approle_role_id        string
	approle_secret_id_file string

	// the locations / field names of the database credentials
	database_path string
}

// Sets up the connection to the Hashicorp Vault
// If you run the Vault in the dev mode, then path should be "secret/"
func New() (*Vault, *hashicorp.Secret, error) {
	if !env.Exists("SDS_VAULT_HOST") {
		return nil, nil, errors.New("missing 'SDS_VAULT_HOST' environment variable")
	}
	if !env.Exists("SDS_VAULT_PORT") {
		return nil, nil, errors.New("missing 'SDS_VAULT_PORT' environment variable")
	}
	if !env.Exists("SDS_VAULT_SECURE") {
		return nil, nil, errors.New("missing 'SDS_VAULT_SECURE' environment variable")
	}

	if !env.Exists("SDS_VAULT_DATABASE_PATH") {
		return nil, nil, errors.New("missing 'SDS_VAULT_DATABASE_PATH' environment variable")
	}

	if !env.Exists("SDS_VAULT_PATH") {
		return nil, nil, errors.New("missing 'SDS_VAULT_PATH' environment variable")
	}

	secure := env.GetString("SDS_VAULT_SECURE")
	if secure != "false" && secure != "true" {
		return nil, nil, errors.New("the value of 'SDS_VAULT_SECURE' could be 'false' or 'true'")
	}

	host := env.GetString("SDS_VAULT_HOST")
	port := env.GetString("SDS_VAULT_PORT")
	path := env.GetString("SDS_VAULT_PATH")
	database_path := env.GetString("SDS_VAULT_DATABASE_PATH")
	approle_role_id := ""
	approle_secret_id_file := ""

	config := hashicorp.DefaultConfig()
	if secure == "true" {
		config.Address = fmt.Sprintf("https://%s:%s", host, port)

		// AppRole RoleID to log in to Vault
		if !env.Exists("SDS_VAULT_APPROLE_ROLE_ID") {
			return nil, nil, errors.New("missing 'SDS_VAULT_APPROLE_ROLE_ID' environment variable")
		}
		approle_role_id = env.GetString("SDS_VAULT_APPROLE_ROLE_ID")

		// AppRole SecretID file path to log in to Vault
		if !env.Exists("SDS_VAULT_APPROLE_SECRET_ID_FILE") {
			return nil, nil, errors.New("missing 'SDS_VAULT_APPROLE_SECRET_ID_FILE' environment variable")
		}

		approle_secret_id_file = env.GetString("SDS_VAULT_APPROLE_SECRET_ID_FILE")
	} else {
		config.Address = fmt.Sprintf("http://%s:%s", host, port)

		if !env.Exists("SDS_VAULT_TOKEN") {
			return nil, nil, errors.New("missing 'SDS_VAULT_TOKEN' environment variable")
		}
	}

	client, err := hashicorp.NewClient(config)
	if err != nil {
		return nil, nil, err
	}

	ctx := context.TODO()

	vault := Vault{
		client:                 client,
		context:                ctx,
		path:                   path,
		database_path:          database_path,
		approle_role_id:        approle_role_id,
		approle_secret_id_file: approle_secret_id_file,
	}

	if secure == "true" {
		token, err := vault.login(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("vault login error: %w", err)
		}

		log.Println("connecting to vault: success!")

		return &vault, token, nil
	} else {
		client.SetToken(env.GetString("SDS_VAULT_TOKEN"))

		return &vault, nil, nil
	}
}

// A combination of a RoleID and a SecretID is required to log into Vault
// with AppRole authentication method. The SecretID is a value that needs
// to be protected, so instead of the app having knowledge of the SecretID
// directly, we have a trusted orchestrator (simulated with a script here)
// give the app access to a short-lived response-wrapping token.
//
// ref: https://www.vaultproject.io/docs/concepts/response-wrapping
// ref: https://learn.hashicorp.com/tutorials/vault/secure-introduction?in=vault/app-integration#trusted-orchestrator
// ref: https://learn.hashicorp.com/tutorials/vault/approle-best-practices?in=vault/auth-methods#secretid-delivery-best-practices
func (v *Vault) login(ctx context.Context) (*hashicorp.Secret, error) {
	log.Printf("logging in to vault with approle auth; role id: %s", v.approle_role_id)

	approleSecretID := &approle.SecretID{
		FromFile: v.approle_secret_id_file,
	}

	appRoleAuth, err := approle.NewAppRoleAuth(
		v.approle_role_id,
		approleSecretID,
		approle.WithWrappingToken(), // only required if the SecretID is response-wrapped
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize approle authentication method: %w", err)
	}

	authInfo, err := v.client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to login using approle auth method: %w", err)
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no approle info was returned after login")
	}

	log.Println("logging in to vault with approle auth: success!")

	return authInfo, nil
}

// Returns the String in the secret, by key
func (v *Vault) GetString(secret_name string, key string) (string, error) {
	secret, err := v.client.KVv2(v.path).Get(v.context, secret_name)
	if err != nil {
		return "", err
	}

	value, ok := secret.Data[key].(string)
	if !ok {
		fmt.Println(secret)
		return "", fmt.Errorf("vault error. failed to get the key %T %#v", secret.Data[key], secret.Data[key])
	}

	return value, nil
}

// GetDatabaseCredentials retrieves a new set of temporary database credentials
func (v *Vault) GetDatabaseCredentials() (db.DatabaseCredentials, *hashicorp.Secret, error) {
	log.Println("getting temporary database credentials from vault")

	lease, err := v.client.Logical().ReadWithContext(v.context, v.database_path)
	if err != nil {
		return db.DatabaseCredentials{}, nil, fmt.Errorf("unable to read secret: %w", err)
	}

	fmt.Println(v.database_path)
	fmt.Println(lease)
	fmt.Println(lease.Data)

	b, err := json.Marshal(lease.Data)
	if err != nil {
		return db.DatabaseCredentials{}, nil, fmt.Errorf("malformed credentials returned: %w", err)
	}

	var credentials db.DatabaseCredentials

	if err := json.Unmarshal(b, &credentials); err != nil {
		return db.DatabaseCredentials{}, nil, fmt.Errorf("unable to unmarshal credentials: %w", err)
	}

	log.Println("getting temporary database credentials from vault: success!")

	// raw secret is included to renew database credentials
	return credentials, lease, nil
}
