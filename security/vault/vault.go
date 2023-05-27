// Package vault creates a service that acts as a gateway
// between hashicorp vault and SDS services.
package vault

import (
	"context"
	"errors"
	"fmt"

	"github.com/blocklords/sds/app/communication/command"
	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/parameter"
	remote_parameter "github.com/blocklords/sds/app/remote/parameter"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db/handler"
	hashicorp "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
)

// Vault is the wrapper around hashicorp vault client along with
// the secret key paths specific for SDS services.
type Vault struct {
	logger         log.Logger
	client         *hashicorp.Client
	database_vault *DatabaseVault
	path           string // Key-Value credentials

	// connection parameters
	approle_role_id    string
	approle_secret_id  string
	approle_mount_path string

	app_config *configuration.Config

	// when vault is launched in the security
	// we call the app role parameters
	// the app role parameters should be renewed later
	auth_token *hashicorp.Secret
}

// VaultConfigurations are setting the default configuration parameters.
//
// The values are the default values if it wasn't provided by the user
// Set the default value to nil, if the parameter is required from the user
var VaultConfigurations = configuration.DefaultConfig{
	Title: "Vault",
	Parameters: key_value.New(map[string]interface{}{
		"SDS_VAULT_HOST":               "localhost",
		"SDS_VAULT_PORT":               8200,
		"SDS_VAULT_HTTPS":              false,
		"SDS_VAULT_APPROLE_MOUNT_PATH": "sds-approle",
		"SDS_VAULT_PATH":               "sds-auth-kv",
		"SDS_VAULT_APPROLE_ROLE_ID":    nil,
		"SDS_VAULT_APPROLE_SECRET_ID":  nil,
	}),
}

// New vault that's connected to remote the Hashicorp Vault.
//
// If you run the Vault in the dev mode, then path should be "sds-auth-kv/"
//
// Optionally the app configuration could be nil, in that case it creates a new vault.
//
// This function also gets the database credentials and pushes them to the
// blockchain.
func New(app_config *configuration.Config, logger log.Logger) (*Vault, error) {
	if app_config == nil {
		return nil, errors.New("missing configuration file")
	}
	// AppRole RoleID to log in to Vault
	if !app_config.Exist("SDS_VAULT_APPROLE_ROLE_ID") {
		return nil, fmt.Errorf("missing 'SDS_VAULT_APPROLE_ROLE_ID' environment variable")
	}
	// AppRole SecretID file path to log in to Vault
	if !app_config.Exist("SDS_VAULT_APPROLE_SECRET_ID") {
		return nil, fmt.Errorf("secure, missing 'SDS_VAULT_APPROLE_SECRET_ID' environment variable")
	}
	if !app_config.Exist("SDS_VAULT_APPROLE_MOUNT_PATH") {
		return nil, fmt.Errorf("secure, missing 'SDS_VAULT_APPROLE_MOUNT_PATH' environment variable")
	}

	vault_logger, err := logger.Child("vault")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	secure := app_config.GetBool("SDS_VAULT_HTTPS")
	host := app_config.GetString("SDS_VAULT_HOST")
	port := app_config.GetString("SDS_VAULT_PORT")

	config := hashicorp.DefaultConfig()
	if secure {
		config.Address = fmt.Sprintf("https://%s:%s", host, port)
	} else {
		config.Address = fmt.Sprintf("http://%s:%s", host, port)
	}

	client, err := hashicorp.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("hashicorp.NewClient: %w", err)
	}

	vault := Vault{
		client:             client,
		logger:             vault_logger,
		path:               app_config.GetString("SDS_VAULT_PATH"),
		approle_mount_path: app_config.GetString("SDS_VAULT_APPROLE_MOUNT_PATH"),
		approle_role_id:    app_config.GetString("SDS_VAULT_APPROLE_ROLE_ID"),
		approle_secret_id:  app_config.GetString("SDS_VAULT_APPROLE_SECRET_ID"),
	}

	ctx, cancel_func := remote_parameter.NewContextWithTimeout(context.TODO(), app_config)
	token, err := vault.login(ctx)
	cancel_func()
	if err != nil {
		return nil, fmt.Errorf("vault login error: %w", err)
	}

	// creates a database credentials wrapper as well
	vault_database, err := NewDatabase(&vault)
	if err != nil {
		return nil, fmt.Errorf("vault create database error: %w", err)
	}

	vault.database_vault = vault_database
	vault.auth_token = token

	credentials, err := vault.get_db_credentials()
	if err != nil {
		return nil, fmt.Errorf("vault get database error: %w", err)
	}

	// Push the credentials to the database engine
	database_client, err := handler.PushSocket()
	if err != nil {
		vault_logger.Fatal("handler.PushSocket", "error", err)
	}
	if err := handler.NEW_CREDENTIALS.Push(database_client, credentials); err != nil {
		vault_logger.Fatal("database socket push", "error", err) // simplified error handling
	}
	err = database_client.Close()
	if err != nil {
		vault_logger.Fatal("database socket close", "error", err) // simplified error handling
	}

	return &vault, nil
}

// Run the vault engine
// along with controller, and automatically renew vault token, database token
func (vault *Vault) Run() {
	go vault.run_controller()
	go vault.periodically_renew_leases()
	go vault.periodically_renew_database_leases()
}

// The vault runs on its own thread.
// The controller is for other threads that wants to read data from the vault
//
// use controller.Controller through controller.NewReply()
func (v *Vault) run_controller() {
	service, err := parameter.InprocessFromUrl(VaultEndpoint())
	if err != nil {
		v.logger.Fatal("parameter.InprocessFromUrl", "error", err.Error())
	}
	reply, err := controller.NewReply(service, v.logger)
	if err != nil {
		v.logger.Fatal("controller.NewReply", "error", err.Error())
	}

	handlers := command.EmptyHandlers().
		Add(GET_STRING, on_get_string)

	reply.Run(handlers, v)
}

// on_get_string returns the string value from vault at bucket/key path.
func on_get_string(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	bucket, err := request.Parameters.GetString("bucket")
	if err != nil {
		return message.Fail("missing bucket parameter")
	}
	key, err := request.Parameters.GetString("key")
	if err != nil {
		return message.Fail("missing key parameter")
	}
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("atleast vault should be given, no parameters")
	}
	v, ok := parameters[0].(*Vault)
	if !ok {
		return message.Fail("parameter is not a vault")
	}

	value, err := v.get_string(bucket, key)

	if err != nil {
		fail := message.Fail("invalid vault request: " + err.Error())
		return fail
	}

	reply := message.Reply{
		Status:  message.OK,
		Message: "",
		Parameters: key_value.Empty().
			Set("value", value),
	}

	return reply
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
	v.logger.Info("Vault login: begin")

	approleSecretID := &approle.SecretID{
		FromString: v.approle_secret_id,
	}

	appRoleAuth, err := approle.NewAppRoleAuth(
		v.approle_role_id,
		approleSecretID,
		approle.WithMountPath(v.approle_mount_path),
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

	v.logger.Info("Vault login: success!")

	return authInfo, nil
}

// Returns the String in the secret, by key
func (v *Vault) get_string(secret_name string, key string) (string, error) {
	ctx, cancel_func := remote_parameter.NewContextWithTimeout(context.TODO(), v.app_config)
	defer cancel_func()

	secret, err := v.client.KVv2(v.path).Get(ctx, secret_name)
	if err != nil {
		return "", fmt.Errorf("vault.client.Get: %w", err)
	}

	value, ok := secret.Data[key].(string)
	if !ok {
		fmt.Println(secret)
		return "", fmt.Errorf("vault error. failed to get the key %T %#v", secret.Data[key], secret.Data[key])
	}

	return value, nil
}
