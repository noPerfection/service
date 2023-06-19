package smartcontract

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/service/configuration"
	parameter "github.com/blocklords/sds/service/identity"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/remote"
	"github.com/blocklords/sds/storage/abi"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSmartcontractDbSuite struct {
	suite.Suite
	db_name       string
	smartcontract Smartcontract
	container     *mysql.MySQLContainer
	db_con        *remote.ClientSocket
	ctx           context.Context
}

func (suite *TestSmartcontractDbSuite) SetupTest() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	storage_abi := "20230308171023_storage_abi.sql"
	storage_smartcontract := "20230308173919_storage_smartcontract.sql"
	abi_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", storage_abi)
	smartcontract_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", storage_smartcontract)
	suite.T().Log("storage smartcontract sql table path", smartcontract_sql_path)

	// run the container
	ctx := context.TODO()
	container, err := mysql.RunContainer(ctx,
		mysql.WithDatabase(suite.db_name),
		mysql.WithUsername("root"),
		mysql.WithPassword("tiger"),
		mysql.WithScripts(abi_sql_path, smartcontract_sql_path),
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

	// Overwrite the default parameters to use test container
	host, err := container.Host(ctx)
	suite.Require().NoError(err)
	ports, err := container.Ports(ctx)
	suite.Require().NoError(err)
	exposed_port := ports["3306/tcp"][0].HostPort

	db.DatabaseConfigurations.Parameters["SDS_DATABASE_HOST"] = host
	db.DatabaseConfigurations.Parameters["SDS_DATABASE_PORT"] = exposed_port
	db.DatabaseConfigurations.Parameters["SDS_DATABASE_NAME"] = suite.db_name

	go db.Run(app_config, logger)
	// wait for initiation of the controller
	time.Sleep(time.Second * 1)

	database_service, err := parameter.Inprocess(parameter.DATABASE)
	suite.Require().NoError(err)
	client, err := remote.InprocRequestSocket(database_service.Url(), logger, app_config)
	suite.Require().NoError(err)

	suite.db_con = client

	key, _ := smartcontract_key.New("1", "0xaddress")
	abi_id := "base64="
	tx_key := blockchain.TransactionKey{
		Id:    "0xtx_id",
		Index: 0,
	}
	header, _ := blockchain.NewHeader(uint64(1), uint64(23))
	deployer := "0xahmetson"

	suite.smartcontract = Smartcontract{
		SmartcontractKey: key,
		AbiId:            abi_id,
		TransactionKey:   tx_key,
		BlockHeader:      header,
		Deployer:         deployer,
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

func (suite *TestSmartcontractDbSuite) TestSmartcontract() {
	var smartcontracts []*Smartcontract
	err := suite.smartcontract.SelectAll(suite.db_con, &smartcontracts)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 0)

	// Insert into the database
	// it should fail, since the smartcontract depends on the
	// abi
	err = suite.smartcontract.Insert(suite.db_con)
	suite.Require().Error(err)

	sample_abi := abi.Abi{
		Bytes: []byte("[{}]"),
		Id:    suite.smartcontract.AbiId,
	}
	err = sample_abi.Insert(suite.db_con)
	suite.Require().NoError(err)

	// inserting a smartcontract should be successful
	err = suite.smartcontract.Insert(suite.db_con)
	suite.Require().NoError(err)

	// duplicate key in the database
	// it should fail
	err = suite.smartcontract.Insert(suite.db_con)
	suite.Require().Error(err)

	// all from database
	err = suite.smartcontract.SelectAll(suite.db_con, &smartcontracts)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 1)
	suite.Require().EqualValues(suite.smartcontract, *smartcontracts[0])
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmartcontractDb(t *testing.T) {
	suite.Run(t, new(TestSmartcontractDbSuite))
}
