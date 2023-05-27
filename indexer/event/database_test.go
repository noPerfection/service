package event

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/indexer/smartcontract"
	"github.com/blocklords/sds/service/configuration"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/parameter"
	"github.com/blocklords/sds/service/remote"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestEventDbSuite struct {
	suite.Suite
	db_name         string
	smartcontract   smartcontract.Smartcontract
	smartcontract_1 smartcontract.Smartcontract
	log             Log
	container       *mysql.MySQLContainer
	db_con          *remote.ClientSocket
	ctx             context.Context
}

func (suite *TestEventDbSuite) SetupTest() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	// event table depends on the smartcontract table
	smartcontract_sql_name := "20230308174318_indexer_smartcontract.sql"
	smartcontract_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", smartcontract_sql_name)

	event_sql_name := "20230308174720_indexer_event.sql"
	event_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", event_sql_name)

	// run the container
	ctx := context.TODO()
	container, err := mysql.RunContainer(ctx,
		mysql.WithDatabase(suite.db_name),
		mysql.WithUsername("root"),
		mysql.WithPassword("tiger"),
		mysql.WithScripts(smartcontract_sql_path, event_sql_path),
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
	suite.smartcontract = smartcontract.Smartcontract{
		SmartcontractKey: key,
		BlockHeader:      header,
	}
	err = suite.smartcontract.Insert(suite.db_con)
	suite.Require().NoError(err)

	key, _ = smartcontract_key.New("12", "0xaddress")
	header, _ = blockchain.NewHeader(uint64(1), uint64(23))
	suite.smartcontract_1 = smartcontract.Smartcontract{
		SmartcontractKey: key,
		BlockHeader:      header,
	}
	err = suite.smartcontract_1.Insert(suite.db_con)
	suite.Require().NoError(err)

	suite.log = Log{
		SmartcontractKey: key,
		BlockHeader:      header,
		TransactionKey: blockchain.TransactionKey{
			Id:    "txid",
			Index: 0,
		},
		Index:      1,
		Name:       "Transfer",
		Parameters: key_value.Empty().Set("value", "1"),
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

func (suite *TestEventDbSuite) TestSmartcontract() {
	///////////////////////////////////////////////////////////////////////
	//
	// Save and GetAll
	//
	///////////////////////////////////////////////////////////////////////

	// inserting a smartcontract should be successful
	err := suite.log.Insert(suite.db_con)
	suite.Require().NoError(err)

	// duplicate key in the database
	// it should fail
	err = suite.log.Insert(suite.db_con)
	suite.Require().Error(err)

	///////////////////////////////////////////////////////////////////////
	//
	// Select by filter
	//
	///////////////////////////////////////////////////////////////////////
	var events []Log

	condition := key_value.Empty().
		Set("smartcontract_keys", []smartcontract_key.Key{suite.smartcontract.SmartcontractKey}).
		Set("block_timestamp", blockchain.Timestamp(1)).
		Set("limit", uint64(500))

	err = suite.log.SelectAllByCondition(suite.db_con, condition, &events)
	suite.Require().NoError(err)
	suite.Require().Len(events, 0)

	condition.Set("smartcontract_keys", []smartcontract_key.Key{suite.smartcontract_1.SmartcontractKey})
	err = suite.log.SelectAllByCondition(suite.db_con, condition, &events)
	suite.Require().NoError(err)
	suite.Require().Len(events, 1)
	suite.Require().EqualValues(suite.log, events[0])
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestEventDb(t *testing.T) {
	suite.Run(t, new(TestEventDbSuite))
}
