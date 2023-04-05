package smartcontract

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"
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
	db_con        *db.Database
	ctx           context.Context
}

func (suite *TestSmartcontractDbSuite) SetupTest() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	static_abi := "20230308171023_static_abi.sql"
	static_smartcontract := "20230308173919_static_smartcontract.sql"
	abi_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", static_abi)
	smartcontract_sql_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", static_smartcontract)
	suite.T().Log("static smartcontract sql table path", smartcontract_sql_path)

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
		if err := db_con.Close(); err != nil {
			suite.T().Fatalf("failed to terminate database connection: %s", err)
		}
	})
}

func (suite *TestSmartcontractDbSuite) TestSmartcontract() {
	smartcontracts, err := GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 0)

	// Insert into the database
	// it should fail, since the smartcontract depends on the
	// abi
	suite.T().Log("insert into the database")
	err = SetInDatabase(suite.db_con, &suite.smartcontract)
	suite.Require().Error(err)

	sample_abi := abi.Abi{
		Bytes: []byte("[{}]"),
		Id:    suite.smartcontract.AbiId,
	}
	err = abi.SetInDatabase(suite.db_con, &sample_abi)
	suite.Require().NoError(err)

	// inserting a smartcontract should be successful
	err = SetInDatabase(suite.db_con, &suite.smartcontract)
	suite.Require().NoError(err)

	// duplicate key in the database
	// it should fail
	err = SetInDatabase(suite.db_con, &suite.smartcontract)
	suite.Require().Error(err)

	// all from database
	smartcontracts, err = GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(smartcontracts, 1)
	suite.Require().EqualValues(suite.smartcontract, *smartcontracts[0])

	// Check is abi exists in the database
	key, err := smartcontract_key.New("offline", "noname")
	suite.Require().NoError(err)
	exist := ExistInDatabase(suite.db_con, key)
	suite.Require().False(exist)

	// but the one we inserted should be
	exist = ExistInDatabase(suite.db_con, suite.smartcontract.SmartcontractKey)
	suite.True(exist)

	// Select abi from database
	// should fail as abi doesn't exist
	_, err = GetFromDatabase(suite.db_con, key)
	suite.Require().Error(err)

	// select abi that exists
	db_sm, err := GetFromDatabase(suite.db_con, suite.smartcontract.SmartcontractKey)
	suite.Require().NoError(err)

	suite.Require().EqualValues(suite.smartcontract, *db_sm)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmartcontractDb(t *testing.T) {
	suite.Run(t, new(TestSmartcontractDbSuite))
}
