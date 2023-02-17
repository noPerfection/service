// The database package handles all the database operations.
// Note that for now it uses Mysql as a hardcoded data
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/common/data_type/key_value"
	_ "github.com/go-sql-driver/mysql"
)

type DatabaseParameters struct {
	hostname string
	port     string
	name     string
	timeout  time.Duration
}

// DatabaseCredentials is a set of dynamic credentials retrieved from Vault
type DatabaseCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Database struct {
	Connection      *sql.DB
	connectionMutex sync.Mutex
	parameters      DatabaseParameters
}

// The configuration parameters
// The values are the default values if it wasn't provided by the user
// Set the default value to nil, if the parameter is required from the user
var DatabaseConfigurations = configuration.DefaultConfig{
	Title: "Database",
	Parameters: key_value.New(map[string]interface{}{
		"SDS_DATABASE_HOST":     "localhost",
		"SDS_DATABASE_PORT":     "3306",
		"SDS_DATABASE_NAME":     "seascape_sds",
		"SDS_DATABASE_TIMEOUT":  uint64(10),
		"SDS_DATABASE_USERNAME": "root",
		"SDS_DATABASE_PASSWORD": "tiger",
	}),
}

// Database parameters fetched from the environment variable.
// It loads parameters such as:
// - host
// - port
// - name
func GetParameters(app_config *configuration.Config) (*DatabaseParameters, error) {
	timeout := app_config.GetUint64("SDS_DATABASE_TIMEOUT")
	if timeout > 3600 {
		return nil, errors.New("the 'SDS_DATABASE_TIMEOUT' value can not be greater than 3600 (seconds)")
	} else if timeout == 0 {
		return nil, errors.New("the 'SDS_DATABASE_TIMEOUT' can not be zero")
	}

	return &DatabaseParameters{
		hostname: app_config.GetString("SDS_DATABASE_HOST"),
		port:     app_config.GetString("SDS_DATABASE_PORT"),
		name:     app_config.GetString("SDS_DATABASE_NAME"),
		timeout:  time.Duration(timeout) * time.Second,
	}, nil
}

func GetDefaultCredentials(app_config *configuration.Config) DatabaseCredentials {
	return DatabaseCredentials{
		Username: app_config.GetString("SDS_DATABASE_USERNAME"),
		Password: app_config.GetString("SDS_DATABASE_PASSWORD"),
	}
}

// NewDatabase establishes a database connection with the given Vault credentials
func Open(parameters *DatabaseParameters, credentials DatabaseCredentials) (*Database, error) {
	database := &Database{
		Connection:      nil,
		connectionMutex: sync.Mutex{},
		parameters:      *parameters,
	}

	ctx := context.TODO()

	// establish the first connection
	if err := database.Reconnect(ctx, credentials); err != nil {
		return nil, err
	}

	return database, nil
}

// Reconnect will be called periodically to refresh the database connection
// since the dynamic credentials expire after some time, it will:
//  1. construct a connection string using the given credentials
//  2. establish a database connection
//  3. close & replace the existing connection with the new one behind a mutex
func (db *Database) Reconnect(ctx context.Context, credentials DatabaseCredentials) error {
	ctx, cancelContextFunc := context.WithTimeout(ctx, db.parameters.timeout)
	defer cancelContextFunc()

	log.Printf(
		"connecting to %q database tcp://%s:%s with username %q and timeout %s",
		db.parameters.name,
		db.parameters.hostname,
		db.parameters.port,
		credentials.Username,
		db.parameters.timeout,
	)

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?timeout=%s",
		credentials.Username,
		credentials.Password,
		db.parameters.hostname,
		db.parameters.port,
		db.parameters.name,
		db.parameters.timeout.String(),
	)

	connection, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("unable to open database connection: %w", err)
	}

	// wait until the database is ready or timeout expires
	for {
		err = connection.Ping()
		if err == nil {
			break
		}
		select {
		case <-time.After(500 * time.Millisecond):
			continue
		case <-ctx.Done():
			return fmt.Errorf("database connection test failed: %w", err)
		}
	}

	db.closeReplaceConnection(connection)

	log.Printf("connecting to %q database: success!", db.parameters.name)

	return nil
}

func (db *Database) closeReplaceConnection(new *sql.DB) {
	/* */ db.connectionMutex.Lock()
	defer db.connectionMutex.Unlock()

	// close the existing connection, if exists
	if db.Connection != nil {
		_ = db.Connection.Close()
	}

	// replace with a new connection
	db.Connection = new
}

func (db *Database) Close() error {
	/* */ db.connectionMutex.Lock()
	defer db.connectionMutex.Unlock()

	if db.Connection != nil {
		return db.Connection.Close()
	}

	return nil
}

// Query
func (db *Database) Query(ctx context.Context, query string, arguments []interface{}) ([]interface{}, error) {
	db.connectionMutex.Lock()
	defer db.connectionMutex.Unlock()

	rows, err := db.Connection.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute %q query: %w", query, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var results []interface{}

	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan table row for %q query: %w", query, err)
		}
		results = append(results, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after scanning %q query: %w", query, err)
	}

	return results, nil
}
