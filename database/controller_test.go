package database

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blocklords/sds/db/handler"
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
type TestControllerSuite struct {
	suite.Suite
	db_name   string
	container *mysql.MySQLContainer
	client    *remote.ClientSocket
	ctx       context.Context
}

func (suite *TestControllerSuite) SetupTest() {
	suite.db_name = "test"
	_, filename, _, _ := runtime.Caller(0)
	storage_abi_sql := "20230308171023_storage_abi.sql"
	storage_abi_path := filepath.Join(filepath.Dir(filename), "..", "_db", "migrations", storage_abi_sql)

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

	logger, err := log.New("controller-suite", log.WITHOUT_TIMESTAMP)
	suite.Require().NoError(err)
	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)

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
	DatabaseConfigurations.Parameters["SDS_DATABASE_PORT"] = exposed_port
	DatabaseConfigurations.Parameters["SDS_DATABASE_NAME"] = suite.db_name

	go Run(app_config, logger)
	// wait for initiation of the controller
	time.Sleep(time.Second * 1)

	database_service, err := parameter.Inprocess(parameter.DATABASE)
	suite.Require().NoError(err)
	client, err := remote.InprocRequestSocket(database_service.Url(), logger, app_config)
	suite.Require().NoError(err)

	suite.client = client

	suite.T().Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			suite.T().Fatalf("failed to terminate container: %s", err)
		}
		if err := client.Close(); err != nil {
			suite.T().Fatalf("failed to close client socket: %s", err)
		}
	})
}

func (suite *TestControllerSuite) TestInsert() {
	suite.T().Log("test INSERT command")
	// query
	arguments := []interface{}{"test_id", `[{}]`}
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"abi_id", "body"},
		Tables:    []string{"storage_abi"},
		Arguments: arguments,
	}
	var reply handler.InsertReply
	err := handler.INSERT.Request(suite.client, request, &reply)
	suite.Require().NoError(err)

	// query
	arguments = []interface{}{"test_id"}
	request = handler.DatabaseQueryRequest{
		Fields:    []string{"abi_id"},
		Tables:    []string{"storage_abi"},
		Where:     "abi_id = ?",
		Arguments: arguments,
	}
	var read_reply handler.SelectRowReply
	err = handler.SELECT_ROW.Request(suite.client, request, &read_reply)
	suite.Require().NoError(err)
	suite.Require().EqualValues("test_id", read_reply.Outputs["abi_id"])

	suite.T().Log("test SELECT ALL command")
	// query
	request = handler.DatabaseQueryRequest{
		Fields: []string{"abi_id", "body"},
		Tables: []string{"storage_abi"},
	}
	var reply_all handler.SelectAllReply
	err = handler.SELECT_ALL.Request(suite.client, request, &reply_all)
	suite.Require().NoError(err)
	suite.T().Log(reply_all)
	suite.Require().Len(reply_all.Rows, 1)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestController(t *testing.T) {
	suite.Run(t, new(TestControllerSuite))
}
