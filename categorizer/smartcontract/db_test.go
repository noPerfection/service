package smartcontract

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/blockchain"
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
	static_smartcontract := "20230308174318_categorizer_smartcontract.sql"
	smartcontract_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", static_smartcontract)
	suite.T().Log("static smartcontract sql table path", smartcontract_sql_path)

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

	database_service, err := service.Inprocess(service.DATABASE)
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

	smartcontracts, err := GetAll(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 0)

	// inserting a smartcontract should be successful
	err = Save(suite.db_con, &suite.smartcontract)
	suite.Require().NoError(err)

	// duplicate key in the database
	// it should fail
	err = Save(suite.db_con, &suite.smartcontract)
	suite.Require().Error(err)

	// all from database
	smartcontracts, err = GetAll(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 1)
	suite.Require().EqualValues(suite.smartcontract, smartcontracts[0])

	///////////////////////////////////////////////////////////////////////
	//
	// GetAllByNetworkId
	//
	///////////////////////////////////////////////////////////////////////
	network_id := "1"
	smartcontracts, err = GetAllByNetworkId(suite.db_con, network_id)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 1)
	suite.Require().EqualValues(suite.smartcontract, smartcontracts[0])

	network_id = "not_existing_id"
	smartcontracts, err = GetAllByNetworkId(suite.db_con, network_id)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 0)

	///////////////////////////////////////////////////////////////////////
	//
	// Get
	//
	///////////////////////////////////////////////////////////////////////

	// get
	sm, err := Get(suite.db_con, suite.smartcontract.SmartcontractKey)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.smartcontract, *sm)

	// can not get the non existing
	_, err = Get(suite.db_con, smartcontract_key.Key{NetworkId: "not_registered", Address: "0xdead"})
	suite.Require().Error(err)
	suite.Require().EqualValues(suite.smartcontract, *sm)

	///////////////////////////////////////////////////////////////////////
	//
	// Exist
	//
	///////////////////////////////////////////////////////////////////////

	// exist
	exist := Exists(suite.db_con, suite.smartcontract.SmartcontractKey)
	suite.Require().True(exist)

	// can not get the non existing
	exist = Exists(suite.db_con, smartcontract_key.Key{NetworkId: "not_registered", Address: "0xdead"})
	suite.Require().False(exist)

	///////////////////////////////////////////////////////////////////////
	//
	// Update parameter then Get
	//
	///////////////////////////////////////////////////////////////////////

	// update
	header, _ := blockchain.NewHeader(uint64(2), uint64(4))
	suite.smartcontract.SetBlockHeader(header)

	err = SaveBlockParameters(suite.db_con, &suite.smartcontract)
	suite.Require().NoError(err)

	// updated smartcontract returned from database
	sm, err = Get(suite.db_con, suite.smartcontract.SmartcontractKey)
	suite.Require().NoError(err)
	suite.Require().EqualValues(header, sm.BlockHeader)

	// updating smartcontract that doesn't exist in the database should fail
	non_exist_sm := Smartcontract{
		SmartcontractKey: smartcontract_key.Key{NetworkId: "not_registered", Address: "0xdead"},
		BlockHeader:      header,
	}
	err = SaveBlockParameters(suite.db_con, &non_exist_sm)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmartcontractDb(t *testing.T) {
	suite.Run(t, new(TestSmartcontractDbSuite))
}
