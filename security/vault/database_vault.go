package vault

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/service/configuration"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/remote/parameter"
	hashicorp "github.com/hashicorp/vault/api"
)

type DatabaseVault struct {
	logger log.Logger
	vault  *Vault
	// the locations / field names of the database credentials
	database_path       string
	database_auth_token *hashicorp.Secret
}

// Default configurations for the database
var DatabaseVaultConfigurations = configuration.DefaultConfig{
	Title: "Database Vault",
	Parameters: key_value.New(map[string]interface{}{
		"SDS_VAULT_DATABASE_PATH": "sds-mysql/creds/sds-mysql-role",
	}),
}

// Create the credentials of the database
func NewDatabase(vault *Vault) (*DatabaseVault, error) {
	vault_logger, err := vault.logger.Child("database")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	database_path := vault.app_config.GetString("SDS_VAULT_DATABASE_PATH")

	database_vault := DatabaseVault{
		vault:         vault,
		logger:        vault_logger,
		database_path: database_path,
	}

	return &database_vault, nil
}

// GetDatabaseCredentials retrieves a new set of temporary database credentials
func (v *Vault) get_db_credentials() (db.DatabaseCredentials, error) {
	v.logger.Info("getting temporary database credentials from vault: begin")

	request_timeout := parameter.RequestTimeout(v.app_config)

	ctx := context.TODO()
	login_ctx, cancel := context.WithTimeout(ctx, request_timeout)
	defer cancel()

	lease, err := v.client.Logical().ReadWithContext(login_ctx, v.database_vault.database_path)
	if err != nil {
		return db.DatabaseCredentials{}, fmt.Errorf("unable to read secret: %w", err)
	}

	b, err := json.Marshal(lease.Data)
	if err != nil {
		return db.DatabaseCredentials{}, fmt.Errorf("malformed credentials returned: %w", err)
	}

	var credentials db.DatabaseCredentials

	if err := json.Unmarshal(b, &credentials); err != nil {
		return db.DatabaseCredentials{}, fmt.Errorf("unable to unmarshal credentials: %w", err)
	}

	v.logger.Info("getting temporary database credentials from vault: success!")

	v.database_vault.database_auth_token = lease

	// raw secret is included to renew database credentials
	return credentials, nil
}
