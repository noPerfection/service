package configuration

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/database"
	"github.com/blocklords/sds/service/configuration"
	parameter "github.com/blocklords/sds/service/identity"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/remote"
	"github.com/blocklords/sds/storage/abi"
	"github.com/blocklords/sds/storage/smartcontract"
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
	db_con        *remote.ClientSocket
	ctx           context.Context
}

func (suite *TestConfigurationDbSuite) SetupTest() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	// configuration depends on smartcontract.
	// smartcontract depends on abi.
	file_dir := filepath.Dir(filename)
	storage_abi := "20230308171023_storage_abi.sql"
	storage_smartcontract := "20230308173919_storage_smartcontract.sql"
	storage_configuration := "20230308173943_storage_configuration.sql"
	change_group_type := "20230314150414_storage_configuration_group_type.sql"

	abi_sql_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", storage_abi)
	smartcontract_sql_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", storage_smartcontract)
	configuration_sql_path := filepath.Join(file_dir, "..", "..", "_db", "migrations", storage_configuration)
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
	app_config.SetDefaults(database.DatabaseConfigurations)

	// Overwrite the default parameters to use test container
	host, err := container.Host(ctx)
	suite.Require().NoError(err)
	ports, err := container.Ports(ctx)
	suite.Require().NoError(err)
	exposed_port := ports["3306/tcp"][0].HostPort

	database.DatabaseConfigurations.Parameters["SDS_DATABASE_HOST"] = host
	database.DatabaseConfigurations.Parameters["SDS_DATABASE_PORT"] = exposed_port
	database.DatabaseConfigurations.Parameters["SDS_DATABASE_NAME"] = suite.db_name

	go database.Run(app_config, logger)
	// wait for initiation of the controller
	time.Sleep(time.Second * 1)

	database_service, err := parameter.Inprocess(parameter.DATABASE)
	suite.Require().NoError(err)
	client, err := remote.InprocRequestSocket(database_service.Url(), logger, app_config)
	suite.Require().NoError(err)

	suite.db_con = client

	// add the storage abi
	abi_id := "base64="
	sample_abi := abi.Abi{
		Bytes: []byte("[{}]"),
		Id:    abi_id,
	}
	err = sample_abi.Insert(suite.db_con)
	suite.Require().NoError(err)

	// add the storage smartcontract
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
	err = sm.Insert(suite.db_con)
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
		if err := suite.db_con.Close(); err != nil {
			suite.T().Fatalf("failed to terminate database connection: %s", err)
		}
	})
}

func (suite *TestConfigurationDbSuite) TestConfiguration() {
	var configs []*Configuration

	err := suite.configuration.SelectAll(suite.db_con, &configs)
	suite.Require().NoError(err)
	suite.Require().Len(configs, 0)

	err = suite.configuration.Insert(suite.db_con)
	suite.Require().NoError(err)

	err = suite.configuration.SelectAll(suite.db_con, &configs)
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
	err = configuration.Insert(suite.db_con)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestConfigurationDb(t *testing.T) {
	suite.Run(t, new(TestConfigurationDbSuite))
}
