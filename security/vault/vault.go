// Keep the credentials in a vault
package vault

import (
	"context"
	"errors"
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/data_type/key_value"
	hashicorp "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"

	zmq "github.com/pebbe/zmq4"
)

type Vault struct {
	logger  log.Logger
	client  *hashicorp.Client
	context context.Context
	path    string // Key-Value credentials

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

// The configuration parameters
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

// Sets up the connection to the Hashicorp Vault
// If you run the Vault in the dev mode, then path should be "sds-auth-kv/"
//
// Optionally the app configuration could be nil, in that case it creates a new vault
func New(logger log.Logger, app_config *configuration.Config) (*Vault, error) {
	if app_config == nil {
		return nil, errors.New("missing configuration file")
	}
	// AppRole RoleID to log in to Vault
	if !app_config.Exist("SDS_VAULT_APPROLE_ROLE_ID") {
		return nil, errors.New("secure, missing 'SDS_VAULT_APPROLE_ROLE_ID' environment variable")
	}
	// AppRole SecretID file path to log in to Vault
	if !app_config.Exist("SDS_VAULT_APPROLE_SECRET_ID") {
		return nil, errors.New("secure, missing 'SDS_VAULT_APPROLE_SECRET_ID' environment variable")
	}
	if !app_config.Exist("SDS_VAULT_APPROLE_MOUNT_PATH") {
		return nil, errors.New("secure, missing 'SDS_VAULT_APPROLE_MOUNT_PATH' environment variable")
	}

	vault_logger := logger.Child("vault", log.WITHOUT_REPORT_CALLER, log.WITH_TIMESTAMP)

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

	ctx := context.TODO()

	vault := Vault{
		client:             client,
		logger:             vault_logger,
		context:            ctx,
		path:               app_config.GetString("SDS_VAULT_PATH"),
		approle_mount_path: app_config.GetString("SDS_VAULT_APPROLE_MOUNT_PATH"),
		approle_role_id:    app_config.GetString("SDS_VAULT_APPROLE_ROLE_ID"),
		approle_secret_id:  app_config.GetString("SDS_VAULT_APPROLE_SECRET_ID"),
	}

	token, err := vault.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("vault login error: %w", err)
	}

	vault.auth_token = token
	return &vault, nil
}

// The vault runs on its own thread.
// The controller is for other threads that wants to read data from the vault
func (v *Vault) RunController() {
	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		v.logger.Fatal("failed to create a new socket", "error", err.Error())
	}

	if err := socket.Bind("inproc://sds_vault"); err != nil {
		v.logger.Fatal("failed to bind to socket", "error", err.Error())
	}

	for {
		msgs, _ := socket.RecvMessage(0)

		// All request types derive from the basic request.
		// We first attempt to parse basic request from the raw message
		request, _ := message.ParseRequest(msgs)

		bucket, _ := request.Parameters.GetString("bucket")
		key, _ := request.Parameters.GetString("key")

		if request.Command == "GetString" {
			value, err := v.get_string(bucket, key)

			if err != nil {
				fail := message.Fail("invalid smartcontract developer request " + err.Error())
				reply_string, _ := fail.ToString()
				if _, err := socket.SendMessage(reply_string); err != nil {
					v.logger.Fatal("failed send", "error", err.Error())
				}
			} else {
				reply := message.Reply{
					Status:  "OK",
					Message: "",
					Parameters: map[string]interface{}{
						"value": value,
					},
				}

				reply_string, _ := reply.ToString()
				if _, err := socket.SendMessage(reply_string); err != nil {
					v.logger.Fatal("failed send", "error", err.Error())
				}
			}
		} else {
			if _, err := socket.SendMessage("vault doesnt support this kind of command"); err != nil {
				v.logger.Fatal("failed send", "error", err.Error())
			}
		}
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
	secret, err := v.client.KVv2(v.path).Get(v.context, secret_name)
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
