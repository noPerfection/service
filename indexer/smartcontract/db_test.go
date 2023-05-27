package smartcontract

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/parameter"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
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
	storage_smartcontract := "20230308174318_indexer_smartcontract.sql"
	smartcontract_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", storage_smartcontract)
	suite.T().Log("storage smartcontract sql table path", smartcontract_sql_path)

	// run the container
	ctx := context.TODO()
	container, err := mysql.RunContainer(ctx,
		mysql.WithDatabase(suite.db_name),
		mysql.WithUsername("root"),
		mysql.WithPassword("tiger"),
		mysql.WithScripts(smartcontract_sql_path),
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
	header, _ := blockchain.NewHeader(uint64(1), uint64(23))

	suite.smartcontract = Smartcontract{
		SmartcontractKey: key,
		BlockHeader:      header,
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
	///////////////////////////////////////////////////////////////////////
	//
	// Save and GetAll
	//
	///////////////////////////////////////////////////////////////////////

	var smartcontracts []Smartcontract
	err := suite.smartcontract.SelectAll(suite.db_con, &smartcontracts)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 0)

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
	suite.Require().EqualValues(suite.smartcontract, smartcontracts[0])

	///////////////////////////////////////////////////////////////////////
	//
	// GetAllByNetworkId
	//
	///////////////////////////////////////////////////////////////////////
	condition := key_value.Empty().Set("network_id", "1")
	err = suite.smartcontract.SelectAllByCondition(suite.db_con, condition, &smartcontracts)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 1)
	suite.Require().EqualValues(suite.smartcontract, smartcontracts[0])

	invalid_condition := key_value.Empty().Set("network_id", "not_existing_id")
	err = suite.smartcontract.SelectAllByCondition(suite.db_con, invalid_condition, &smartcontracts)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 0)

	///////////////////////////////////////////////////////////////////////
	//
	// Get
	//
	///////////////////////////////////////////////////////////////////////

	sm := Smartcontract{
		SmartcontractKey: suite.smartcontract.SmartcontractKey,
	}

	// get
	err = sm.Select(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.smartcontract, sm)

	// can not get the non existing
	invalid_key := smartcontract_key.Key{NetworkId: "not_registered", Address: "0xdead"}
	invalid_sm := Smartcontract{
		SmartcontractKey: invalid_key,
	}
	err = invalid_sm.Select(suite.db_con)
	suite.Require().Error(err)

	///////////////////////////////////////////////////////////////////////
	//
	// Exist
	//
	///////////////////////////////////////////////////////////////////////

	// exist
	exist := suite.smartcontract.Exist(suite.db_con)
	suite.Require().True(exist)

	// can not get the non existing
	exist = invalid_sm.Exist(suite.db_con)
	suite.Require().False(exist)

	///////////////////////////////////////////////////////////////////////
	//
	// Update parameter then Get
	//
	///////////////////////////////////////////////////////////////////////

	// update
	header, _ := blockchain.NewHeader(uint64(2), uint64(4))
	suite.smartcontract.SetBlockHeader(header)

	err = suite.smartcontract.Update(suite.db_con, UPDATE_BLOCK_HEADER)
	suite.Require().NoError(err)

	// updated smartcontract returned from database
	err = sm.Select(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().EqualValues(header, sm.BlockHeader)

	// updating smartcontract that doesn't exist in the database should fail
	err = invalid_sm.Update(suite.db_con, UPDATE_BLOCK_HEADER)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmartcontractDb(t *testing.T) {
	suite.Run(t, new(TestSmartcontractDbSuite))
}
