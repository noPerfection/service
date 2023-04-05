package abi

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/db"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestAbiDbSuite struct {
	suite.Suite
	db_name   string
	container *mysql.MySQLContainer
	db_con    *db.Database
	ctx       context.Context
}

func (suite *TestAbiDbSuite) SetupTest() {
	// prepare the database creation
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	static_abi_sql := "20230308171023_static_abi.sql"
	static_abi_path := filepath.Join(filepath.Dir(filename), "..", "..", "_db", "migrations", static_abi_sql)

	// run the container
	ctx := context.TODO()
	container, err := mysql.RunContainer(ctx,
		mysql.WithDatabase(suite.db_name),
		mysql.WithUsername("root"),
		mysql.WithPassword("tiger"),
		mysql.WithScripts(static_abi_path),
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

	suite.T().Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			suite.T().Fatalf("failed to terminate container: %s", err)
		}
		if err := db_con.Close(); err != nil {
			suite.T().Fatalf("failed to terminate database connection: %s", err)
		}
	})
}

func (suite *TestAbiDbSuite) TestAbi() {
	abis, err := GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(abis, 0)

	bytes := []byte(`[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"},{"name":"Approval","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"ApprovalForAll","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"operator","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"bool","indexed":false,"internalType":"bool"}],"anonymous":false},{"name":"Minted","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"id","type":"uint256","indexed":true,"internalType":"uint256"},{"name":"generation","type":"uint256","indexed":false,"internalType":"uint256"},{"name":"quality","type":"uint8","indexed":false,"internalType":"uint8"}],"anonymous":false},{"name":"OwnershipTransferred","type":"event","inputs":[{"name":"previousOwner","type":"address","indexed":true,"internalType":"address"},{"name":"newOwner","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address","indexed":true,"internalType":"address"},{"name":"to","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"approve","type":"function","inputs":[{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"balanceOf","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"baseURI","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"burn","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"getApproved","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"isApprovedForAll","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"operator","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"name","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"owner","type":"function","inputs":[],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"ownerOf","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"paramsOf","type":"function","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"quality","type":"uint256","internalType":"uint256"},{"name":"generation","type":"uint8","internalType":"uint8"}],"stateMutability":"view"},{"name":"renounceOwnership","type":"function","inputs":[],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"},{"name":"_data","type":"bytes","internalType":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setApprovalForAll","type":"function","inputs":[{"name":"operator","type":"address","internalType":"address"},{"name":"approved","type":"bool","internalType":"bool"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"supportsInterface","type":"function","inputs":[{"name":"interfaceId","type":"bytes4","internalType":"bytes4"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"symbol","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"tokenByIndex","type":"function","inputs":[{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenOfOwnerByIndex","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenURI","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"totalSupply","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"transferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"transferOwnership","type":"function","inputs":[{"name":"newOwner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"mint","type":"function","inputs":[{"name":"_to","type":"address","internalType":"address"},{"name":"_generation","type":"uint256","internalType":"uint256"},{"name":"_quality","type":"uint8","internalType":"uint8"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"nonpayable"},{"name":"setOwner","type":"function","inputs":[{"name":"_owner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setFactory","type":"function","inputs":[{"name":"_factory","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setBaseUri","type":"function","inputs":[{"name":"_uri","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"}]`)
	abi := Abi{
		Bytes: bytes,
	}
	err = abi.GenerateId()
	suite.Require().NoError(err)

	// Insert into the database
	err = SetInDatabase(suite.db_con, &abi)
	suite.Require().NoError(err)

	// duplicate key in database
	err = SetInDatabase(suite.db_con, &abi)
	suite.Require().Error(err)

	abis, err = GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(abis, 1)
	suite.Require().EqualValues(abi.Id, abis[0].Id)

	// add more data
	bytes = []byte(`[]`)
	abi = Abi{
		Bytes: bytes,
	}
	err = abi.GenerateId()
	suite.Require().NoError(err)
	err = SetInDatabase(suite.db_con, &abi)
	suite.Require().NoError(err)

	abis, err = GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(abis, 2)

	// add more data
	bytes = []byte(`[{}]`)
	abi = Abi{
		Bytes: bytes,
	}
	err = abi.GenerateId()
	suite.Require().NoError(err)
	err = SetInDatabase(suite.db_con, &abi)
	suite.Require().NoError(err)

	abis, err = GetAllFromDatabase(suite.db_con)
	suite.Require().NoError(err)
	suite.Require().Len(abis, 3)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAbiDb(t *testing.T) {
	suite.Run(t, new(TestAbiDbSuite))
}
