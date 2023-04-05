package configuration

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"
	"github.com/blocklords/sds/static/smartcontract"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestConfigurationDbSuite struct {
	suite.Suite
	db_name       string
	configuration Configuration
	container     *mysql.MySQLContainer
	db_con        *db.Database
	ctx           context.Context
}

func (suite *TestConfigurationDbSuite) SetupTest() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	// configuration depends on smartcontract.
	// smartcontract depends on abi.
	file_dir := filepath.Dir(filename)
	static_abi := "20230308171023_static_abi.sql"
	static_smartcontract := "20230308173919_static_smartcontract.sql"
	static_configuration := "20230308173943_static_configuration.sql"
	change_group_type := "20230314150414_static_configuration_group_type.sql"

	abi_sql_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", static_abi)
	smartcontract_sql_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", static_smartcontract)
	configuration_sql_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", static_configuration)
	change_group_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", change_group_type)

	suite.T().Log("the configuration table path", configuration_sql_path)

	// run the container
	ctx := context.TODO()
	container, err := mysql.RunContainer(ctx,
		mysql.WithDatabase(suite.db_name),
		mysql.WithUsername("root"),
		mysql.WithPassword("tiger"),
		mysql.WithScripts(abi_sql_path, smartcontract_sql_path, configuration_sql_path, change_group_path),
	)
	suite.Require().NoError(err)
	suite.container = container
	suite.ctx = ctx

	logger, err := log.New("mysql-suite", log.WITHOUT_TIMESTAMP)
	suite.Require().NoError(err)
	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)

	// Creating a database client
	// after settings the default parameters
	// we should have the user name and password
	app_config.SetDefaults(db.DatabaseConfigurations)
	credentials := db.GetDefaultCredentials(app_config)

	// Overwrite the host
	host, err := container.Host(ctx)
	suite.Require().NoError(err)
	app_config.SetDefault("SDS_DATABASE_HOST", host)

	// Overwrite the port
	ports, err := container.Ports(ctx)
	suite.Require().NoError(err)
	exposed_port := ports["3306/tcp"][0].HostPort
	app_config.SetDefault("SDS_DATABASE_PORT", exposed_port)

	// overwrite the database name
	app_config.SetDefault("SDS_DATABASE_NAME", suite.db_name)
	parameters, err := db.GetParameters(app_config)
	suite.Require().NoError(err)

	// Connect to the database
	db_con, err := db.Open(logger, parameters, credentials)
	suite.Require().NoError(err)
	suite.db_con = db_con

	// // create configuration sql
	// configuration_sql, err := os.ReadFile(configuration_sql_path)
	// suite.Require().NoError(err)
	// // add configuration path
	// arguments := []interface{}{}
	// _, err = suite.db_con.Query(suite.ctx, string(configuration_sql), arguments)
	// suite.Require().NoError(err)

	// add the static abi
	abi_id := "base64="
	sample_abi := abi.Abi{
		Bytes: []byte("[{}]"),
		Id:    abi_id,
	}
	err = abi.SetInDatabase(suite.db_con, &sample_abi)
	suite.Require().NoError(err)

	// add the static smartcontract
	key, _ := smartcontract_key.New("1", "0xaddress")
	tx_key := blockchain.TransactionKey{
		Id:    "0xtx_id",
		Index: 0,
	}
	header, _ := blockchain.NewHeader(uint64(1), uint64(23))
	deployer := "0xahmetson"

	sm := smartcontract.Smartcontract{
		SmartcontractKey: key,
		AbiId:            abi_id,
		TransactionKey:   tx_key,
		BlockHeader:      header,
		Deployer:         deployer,
	}
	err = smartcontract.SetInDatabase(suite.db_con, &sm)
	suite.Require().NoError(err)

	sample := topic.Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
	}
	suite.configuration = Configuration{
		Topic:   sample,
		Address: key.Address,
	}

	suite.T().Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			suite.T().Fatalf("failed to terminate container: %s", err)
		}
		if err := db_con.Close(); err != nil {
			suite.T().Fatalf("failed to terminate database connection: %s", err)
		}
	})
}

func (suite *TestConfigurationDbSuite) TestConfiguration() {
	configs, err := GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(configs, 0)

	err = SetInDatabase(suite.db_con, &suite.configuration)
	suite.Require().NoError(err)

	configs, err = GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(configs, 1)
	suite.Require().EqualValues(suite.configuration, *configs[0])

	// inserting a configuration
	// that links to the non existing smartcontract
	// should fail
	sample := topic.Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestToken",
	}
	configuration := Configuration{
		Topic:   sample,
		Address: "not_inserted",
	}
	err = SetInDatabase(suite.db_con, &configuration)
	suite.Require().Error(err)

	// should fail since we don't have this configuration
	exist := ExistInDatabase(suite.db_con, &configuration.Topic)
	suite.Require().False(exist)

	// but the one we inserted should be
	exist = ExistInDatabase(suite.db_con, &suite.configuration.Topic)
	suite.True(exist)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestConfigurationDb(t *testing.T) {
	suite.Run(t, new(TestConfigurationDbSuite))
}
