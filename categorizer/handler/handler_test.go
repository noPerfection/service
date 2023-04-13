package handler

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	blockchain_command "github.com/blocklords/sds/blockchain/handler"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
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
type TestHandlerSuite struct {
	suite.Suite
	app_config *configuration.Config
	logger     log.Logger

	sm_0_key smartcontract_key.Key
	sm_1_key smartcontract_key.Key
	sm_0     smartcontract.Smartcontract
	sm_1     smartcontract.Smartcontract

	db_name   string
	container *mysql.MySQLContainer
	db_con    *remote.ClientSocket
	ctx       context.Context

	clients    key_value.KeyValue // clients to pass as app parameters to the command handlers
	evm_router *controller.Router
	networks   network.Networks

	logs key_value.List
}

func (suite *TestHandlerSuite) setup_network_service() {
	suite.app_config.SetDefault(network.SDS_BLOCKCHAIN_NETWORKS, network.DefaultConfiguration())
	// router services
	evm_router_service, err := service.NewExternal(service.EVM, service.THIS, suite.app_config)
	suite.Require().NoError(err, "failed to create categorizer service")

	// Run the background Reply Controllers
	// Router's dealers will connect to them
	network_id_1 := "1"
	network_1_categorizer_url := inproc.CategorizerEndpoint(network_id_1)
	network_1_categorizer_service, _ := service.InprocessFromUrl(network_1_categorizer_url)
	network_1_categorizer_reply, _ := controller.NewReply(network_1_categorizer_service, suite.logger)

	clients, _ := network.NewClientSockets(suite.app_config, suite.logger)
	suite.clients = clients

	////////////////////////////////////////////////////////
	//
	// Define the sockets
	//
	////////////////////////////////////////////////////////
	evm_router, err := controller.NewRouter(evm_router_service, suite.logger)
	suite.Require().NoError(err)
	suite.evm_router = &evm_router
	suite.networks, _ = network.GetNetworks(suite.app_config, network.EVM)

	////////////////////////////////////////////////////
	//
	// The remote network service command handlers
	//
	////////////////////////////////////////////////////

	command_1 := blockchain_command.NEW_CATEGORIZED_SMARTCONTRACTS
	command__1_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		suite.logger.Info("reply back command", "service", service.CATEGORIZER)
		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("id", command_1.String()).
				Set("dealer", service.CATEGORIZER.ToString()),
		}
	}

	categorizer_handlers := command.EmptyHandlers().
		Add(command_1, command__1_handler)

	///////////////////////////////////////////////////////////////
	// The network sub services
	// The categorization and client
	//
	// Categorization sends the command to the categorizer
	//
	// Client sends the command to the remote blockchain
	go network_1_categorizer_reply.Run(categorizer_handlers)

	err = suite.evm_router.AddDealers(
		network_1_categorizer_service,
	)
	suite.Require().NoError(err, "failed to add dealer, because limit is THIS")
	go suite.evm_router.Run()

	// Prepare for the controllers to be ready
	time.Sleep(time.Millisecond * 200)
}

func (suite *TestHandlerSuite) setup_app() {
	logger, err := log.New("test_handler", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)
	suite.logger = logger

	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)
	suite.app_config = app_config
}

// for given index i of the log, calculate timestamps
func (suite *TestHandlerSuite) calculate_timestamp(i int) uint64 {
	return uint64(5 * (1 + i))
}

func (suite *TestHandlerSuite) setup_db() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	static_smartcontract := "20230308174318_categorizer_smartcontract.sql"
	smartcontract_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", static_smartcontract)

	event_sql_name := "20230308174720_categorizer_event.sql"
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

	// Creating a database client
	// after settings the default parameters
	// we should have the user name and password
	suite.app_config.SetDefaults(db.DatabaseConfigurations)

	// Overwrite the default parameters to use test container
	host, err := container.Host(ctx)
	suite.Require().NoError(err)
	ports, err := container.Ports(ctx)
	suite.Require().NoError(err)
	exposed_port := ports["3306/tcp"][0].HostPort

	db.DatabaseConfigurations.Parameters["SDS_DATABASE_HOST"] = host
	db.DatabaseConfigurations.Parameters["SDS_DATABASE_PORT"] = exposed_port
	db.DatabaseConfigurations.Parameters["SDS_DATABASE_NAME"] = suite.db_name

	go db.Run(suite.app_config, suite.logger)
	// wait for initiation of the controller
	time.Sleep(time.Second * 1)

	database_service, err := service.Inprocess(service.DATABASE)
	suite.Require().NoError(err)
	client, err := remote.InprocRequestSocket(database_service.Url(), suite.logger, suite.app_config)
	suite.Require().NoError(err)

	suite.db_con = client

	suite.T().Cleanup(func() {
		if err := suite.db_con.Close(); err != nil {
			suite.T().Fatalf("failed to terminate database connection: %s", err)
		}
		suite.Require().True(suite.container.IsRunning())

		if err := suite.container.Terminate(ctx); err != nil {
			suite.T().Fatalf("failed to terminate container: %s", err)
		}

		suite.Require().False(suite.container.IsRunning())
	})
}

func (suite *TestHandlerSuite) SetupTest() {
	header, _ := blockchain.NewHeader(uint64(1), uint64(22))
	suite.sm_0_key, _ = smartcontract_key.New("1", "0xaddress")
	suite.sm_0 = smartcontract.Smartcontract{
		SmartcontractKey: suite.sm_0_key,
		BlockHeader:      header,
	}

	header, _ = blockchain.NewHeader(uint64(2), uint64(44))
	suite.sm_1_key, _ = smartcontract_key.New("1", "second_contract")
	suite.sm_1 = smartcontract.Smartcontract{
		SmartcontractKey: suite.sm_1_key,
		BlockHeader:      header,
	}

	suite.setup_app()
	suite.setup_db()

	// inserting a smartcontract should be successful
	err := suite.sm_0.Insert(suite.db_con)
	suite.Require().NoError(err)
	err = suite.sm_1.Insert(suite.db_con)
	suite.Require().NoError(err)

	suite.logs = *key_value.NewList()

	// random 10 logs
	for i := 0; i < 10; i++ {
		header, _ := blockchain.NewHeader(uint64(i+1), suite.calculate_timestamp(i))

		log := event.Log{
			SmartcontractKey: suite.sm_0_key,
			BlockHeader:      header,
			TransactionKey: blockchain.TransactionKey{
				Id:    "txid",
				Index: 0,
			},
			Index:      uint(i + 1),
			Name:       "Transfer",
			Parameters: key_value.Empty().Set("value", "1"),
		}
		err := suite.logs.Add(i, log)
		suite.Require().NoError(err)

		err = log.Insert(suite.db_con)
		suite.Require().NoError(err)
	}
	for i := 5; i < 15; i++ {
		header, _ := blockchain.NewHeader(uint64(i+1), suite.calculate_timestamp(i))

		log := event.Log{
			SmartcontractKey: suite.sm_1_key,
			BlockHeader:      header,
			TransactionKey: blockchain.TransactionKey{
				Id:    "txid",
				Index: 0,
			},
			Index:      uint(i + 1 + 5),
			Name:       "Transfer",
			Parameters: key_value.Empty().Set("value", "1"),
		}
		err := suite.logs.Add(i+5, log)
		suite.Require().NoError(err)

		err = log.Insert(suite.db_con)
		suite.Require().NoError(err)

	}
}

// all database operations should be done in a one test
func (suite *TestHandlerSuite) TestCommands() {

	////////////////////////////////////////////////////////
	//
	// GetSmartcontract command
	//
	////////////////////////////////////////////////////////

	// valid request
	valid_kv, err := key_value.NewFromInterface(suite.sm_0_key)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    GET_SMARTCONTRACT.String(),
		Parameters: valid_kv,
	}
	reply := GetSmartcontract(request, suite.logger, suite.db_con, "", "")
	suite.Require().True(reply.IsOK())

	var replied_sm GetSmartcontractReply
	err = reply.Parameters.ToInterface(&replied_sm)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.sm_0, replied_sm.Smartcontract)

	// request with empty parameter should fail
	request = message.Request{
		Command:    GET_SMARTCONTRACT.String(),
		Parameters: key_value.Empty(),
	}
	reply = GetSmartcontract(request, suite.logger, suite.db_con, "", "")
	suite.Require().False(reply.IsOK())

	////////////////////////////////////////////////////////
	//
	// GetSmartcontracts command
	//
	////////////////////////////////////////////////////////

	request = message.Request{
		Command:    GET_SMARTCONTRACT.String(),
		Parameters: key_value.Empty(),
	}
	reply = GetSmartcontracts(request, suite.logger, suite.db_con, "", "")
	suite.Require().True(reply.IsOK())

	var replied_smartcontracts GetSmartcontractsReply
	err = reply.Parameters.ToInterface(&replied_smartcontracts)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.sm_0, replied_smartcontracts.Smartcontracts[0])
	suite.Require().EqualValues(suite.sm_1, replied_smartcontracts.Smartcontracts[1])

	////////////////////////////////////////////////////////
	//
	// SetSmartcontract command
	//
	////////////////////////////////////////////////////////

	suite.setup_network_service()

	network_id_key := smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xnft",
	}

	// valid request
	valid_smartcontract := smartcontract.Smartcontract{
		SmartcontractKey: network_id_key,
		BlockHeader: blockchain.BlockHeader{
			Number:    blockchain.Number(1),
			Timestamp: blockchain.Timestamp(2),
		},
	}
	request_parameters := SetSmartcontractRequest{
		Smartcontract: valid_smartcontract,
	}

	valid_kv, err = key_value.NewFromInterface(request_parameters)
	suite.Require().NoError(err)

	request = message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply = SetSmartcontract(request, suite.logger, suite.db_con, suite.clients, suite.networks)
	suite.Require().True(reply.IsOK())

	// we could return the data
	valid_kv, err = key_value.NewFromInterface(network_id_key)
	suite.Require().NoError(err)

	request = message.Request{
		Command:    GET_SMARTCONTRACT.String(),
		Parameters: valid_kv,
	}
	reply = GetSmartcontract(request, suite.logger, suite.db_con, "", "")

	suite.Require().True(reply.IsOK())

	err = reply.Parameters.ToInterface(&replied_sm)
	suite.Require().NoError(err)
	suite.Require().EqualValues(network_id_key, replied_sm.Smartcontract.SmartcontractKey)

	// registering with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = SetSmartcontract(request, suite.logger, suite.db_con, suite.clients, suite.networks)
	suite.Require().False(reply.IsOK())

	////////////////////////////////////////////////////////
	//
	// GetSnapshot command
	//
	////////////////////////////////////////////////////////

	snapshot_parameters := Snapshot{
		BlockTimestamp:    suite.sm_0.BlockHeader.Timestamp,
		SmartcontractKeys: []smartcontract_key.Key{},
	}
	valid_kv, err = key_value.NewFromInterface(snapshot_parameters)
	suite.Require().NoError(err)
	request = message.Request{
		Command:    SNAPSHOT.String(),
		Parameters: valid_kv,
	}
	// Snapshot should fail since no smartcontract keys were given
	reply = GetSnapshot(request, suite.logger, suite.db_con)
	suite.Require().False(reply.IsOK())

	// Getting snapshot for the first smartcontract key
	snapshot_parameters.SmartcontractKeys = []smartcontract_key.Key{suite.sm_0_key}
	snapshot_parameters.BlockTimestamp, _ = blockchain.NewTimestamp(suite.calculate_timestamp(0))
	valid_kv, err = key_value.NewFromInterface(snapshot_parameters)
	suite.Require().NoError(err)
	request = message.Request{
		Command:    SNAPSHOT.String(),
		Parameters: valid_kv,
	}
	// Snapshot should fail since no smartcontract keys were given
	reply = GetSnapshot(request, suite.logger, suite.db_con)
	suite.Require().True(reply.IsOK())

	var reply_parameters SnapshotReply
	err = reply.Parameters.ToInterface(&reply_parameters)
	suite.Require().NoError(err)

	// we added 10 logs in suite.SetupTest(), should fetch all
	suite.Require().Len(reply_parameters.Logs, 10)
	suite.Require().EqualValues(reply_parameters.BlockTimestamp, suite.calculate_timestamp(9))

	// fetching the data for the non existing timestamp should
	// return empty list
	snapshot_parameters.BlockTimestamp, _ = blockchain.NewTimestamp(suite.calculate_timestamp(10))
	valid_kv, err = key_value.NewFromInterface(snapshot_parameters)
	suite.Require().NoError(err)
	request = message.Request{
		Command:    SNAPSHOT.String(),
		Parameters: valid_kv,
	}
	// Snapshot should fail since no smartcontract keys were given
	reply = GetSnapshot(request, suite.logger, suite.db_con)
	suite.Require().True(reply.IsOK())
	err = reply.Parameters.ToInterface(&reply_parameters)
	suite.Require().NoError(err)

	suite.Require().Len(reply_parameters.Logs, 0)
	suite.Require().EqualValues(reply_parameters.BlockTimestamp, suite.calculate_timestamp(10))

	// fetching all from timestamp of log #5 for two smartcontract keys
	snapshot_parameters.SmartcontractKeys = []smartcontract_key.Key{suite.sm_0_key, suite.sm_1_key}
	snapshot_parameters.BlockTimestamp, _ = blockchain.NewTimestamp(suite.calculate_timestamp(5))

	valid_kv, err = key_value.NewFromInterface(snapshot_parameters)
	suite.Require().NoError(err)
	request = message.Request{
		Command:    SNAPSHOT.String(),
		Parameters: valid_kv,
	}
	// Snapshot should fail since no smartcontract keys were given
	reply = GetSnapshot(request, suite.logger, suite.db_con)
	suite.Require().True(reply.IsOK())
	err = reply.Parameters.ToInterface(&reply_parameters)
	suite.Require().NoError(err)

	suite.Require().Len(reply_parameters.Logs, 15)
	suite.Require().EqualValues(reply_parameters.BlockTimestamp, suite.calculate_timestamp(14))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmartcontract(t *testing.T) {
	suite.Run(t, new(TestHandlerSuite))
}
