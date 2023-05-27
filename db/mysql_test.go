package db

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestMysqlSuite struct {
	suite.Suite
	db_name   string
	container *mysql.MySQLContainer
	db_con    *Database
	ctx       context.Context
}

func (suite *TestMysqlSuite) SetupTest() {
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	storage_abi_sql := "20230308171023_storage_abi.sql"
	storage_abi_path := filepath.Join(filepath.Dir(filename), "..", "_db", "migrations", storage_abi_sql)

	// create_test_db := "create_test_db.sql"
	// create_test_db_path := filepath.Join(filepath.Dir(filename), "..", "_db", create_test_db)

	ctx := context.TODO()
	container, err := mysql.RunContainer(ctx,
		mysql.WithDatabase(suite.db_name),
		mysql.WithUsername("root"),
		mysql.WithPassword("tiger"),
		mysql.WithScripts(storage_abi_path),
	)

	suite.Require().NoError(err)
	suite.container = container
	suite.ctx = ctx

	logger, err := log.New("mysql-suite", log.WITHOUT_TIMESTAMP)
	suite.Require().NoError(err)
	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)

	// Getting default parameters should fail
	// since we don't have any data set yet
	credentials := GetDefaultCredentials(app_config)
	suite.Require().Empty(credentials.Username)
	suite.Require().Empty(credentials.Password)

	// after settings the default parameters
	// we should have the user name and password
	app_config.SetDefaults(DatabaseConfigurations)
	credentials = GetDefaultCredentials(app_config)
	suite.Require().Equal("root", credentials.Username)
	suite.Require().Equal("tiger", credentials.Password)

	// Overwrite the host
	host, err := container.Host(ctx)
	suite.Require().NoError(err)
	app_config.SetDefault("SDS_DATABASE_HOST", host)

	// Overwrite the port
	ports, err := container.Ports(ctx)
	suite.Require().NoError(err)
	exposed_port := ""
	for _, port := range ports {
		if len(ports) > 0 {
			exposed_port = port[0].HostPort
			break
		}
	}
	suite.Require().NotEmpty(exposed_port)
	app_config.SetDefault("SDS_DATABASE_PORT", exposed_port)

	// overwrite the database name
	app_config.SetDefault("SDS_DATABASE_NAME", suite.db_name)
	parameters, err := GetParameters(app_config)
	suite.Require().NoError(err)
	suite.Require().Equal(suite.db_name, parameters.name)

	// Connect to the database
	suite.T().Log("open database connection by", parameters.hostname, credentials)
	db_con, err := Open(logger, parameters, credentials)
	suite.Require().NoError(err)
	suite.db_con = db_con

	suite.T().Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			suite.T().Fatalf("failed to terminate container: %s", err)
		}
		if err := db_con.Close(); err != nil {
			suite.T().Fatalf("failed to terminate database connection: %s", err)
		}
	})
}

func (suite *TestMysqlSuite) TestInsert() {
	// query
	query := `INSERT INTO storage_abi (abi_id, body) VALUES (?, ?)`
	arguments := []interface{}{"test_id", `[{}]`}

	_, err := suite.db_con.Query(suite.ctx, query, arguments)
	suite.Require().NoError(err)

	// query
	query = `SELECT abi_id FROM storage_abi WHERE abi_id = ?`
	arguments = []interface{}{"test_id"}

	_, err = suite.db_con.Query(suite.ctx, query, arguments)
	suite.Require().NoError(err)
}

func (suite *TestMysqlSuite) TestSelect() {
	// query
	query := `SELECT abi_id FROM storage_abi WHERE abi_id = ?`
	arguments := []interface{}{"test_id"}

	result, err := suite.db_con.Query(suite.ctx, query, arguments)
	suite.Require().NoError(err)
	fmt.Println("the select result", result)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestMysql(t *testing.T) {
	suite.Run(t, new(TestMysqlSuite))
}
