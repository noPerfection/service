package service

import (
	win "os"
	"path/filepath"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/os/path"
	"github.com/noPerfection/protocol/client"
	clientConfig "github.com/noPerfection/protocol/client/config"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/manager_client"
	"github.com/noPerfection/protocol/handler/route"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestServiceSuite struct {
	suite.Suite

	service    *Independent // the manager to test
	currentDir string       // executable to store the binaries and source codes
	name       string       // the name of the service
	handler    base.Interface
	logger     *log.Logger

	defaultHandleFunc route.HandleFunc0
	cmd1              string
	handlerCategory   string
}

func (test *TestServiceSuite) createYaml(dir string, name string) {
	s := test.Require

	kv := datatype.New().Set("services", []interface{}{})

	marshalledConfig, err := yaml.Marshal(kv.Map())
	s().NoError(err)

	filePath := filepath.Join(dir, name+".yml")

	f, err := win.OpenFile(filePath, win.O_RDWR|win.O_CREATE|win.O_TRUNC, 0644)
	s().NoError(err)
	_, err = f.Write(marshalledConfig)
	s().NoError(err)

	s().NoError(f.Close())
}

func (test *TestServiceSuite) deleteYaml(dir string, name string) {
	s := test.Require

	filePath := filepath.Join(dir, name+".yml")

	exist, err := path.FileExist(filePath)
	s().NoError(err)

	if !exist {
		return
	}

	s().NoError(win.Remove(filePath))
}

func (test *TestServiceSuite) SetupTest() {
	s := test.Suite.Require

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	test.name = "service_1"

	// handler
	syncReplier := sync_replier.New()
	test.defaultHandleFunc = func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(datatype.New())
	}
	test.cmd1 = "hello"
	s().NoError(syncReplier.Route(test.cmd1, test.defaultHandleFunc))
	test.handler = syncReplier

	test.logger, err = log.New("test", true)
	s().NoError(err)

	test.handlerCategory = "main"
	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, test.handlerCategory)
	test.handler.SetConfig(inprocConfig)
	s().NoError(test.handler.SetLogger(test.logger))
}

func (test *TestServiceSuite) closeService() {
	s := test.Suite.Require
	if test.service != nil {
		s().NoError(test.service.topologyHandler.Close())

		test.service = nil

		// Wait a bit for closing the threads
		time.Sleep(time.Second)
	}

	test.deleteYaml(test.currentDir, "app")
}

func (test *TestServiceSuite) newService() {
	s := test.Suite.Require

	created, err := New(test.name)
	s().NoError(err)

	test.service = created
	test.service.SetHandler(test.handlerCategory, test.handler)
}

func (test *TestServiceSuite) mainHandler() base.Interface {
	return test.service.Handlers["main"].(base.Interface)
}

func (test *TestServiceSuite) externalClient(hConfig *handlerConfig.Handler) *client.Socket {
	s := test.Suite.Require

	// let's test that handler runs
	targetZmqType := handlerConfig.SocketType(hConfig.Type)
	externalConfig := clientConfig.New(test.service.Name(), hConfig.Id, hConfig.Port, targetZmqType)
	externalConfig.UrlFunc(clientConfig.Url)
	externalClient, err := client.New(externalConfig)
	s().NoError(err)

	return externalClient
}

func (test *TestServiceSuite) managerClient() *client.Socket {
	s := test.Suite.Require

	createdConfig, err := test.service.topologyHandler.Config().Service(test.name)
	s().NoError(err)
	managerConfig := createdConfig.Manager
	managerConfig.UrlFunc(clientConfig.Url)
	managerClient, err := client.New(managerConfig)
	s().NoError(err)

	return managerClient
}

// Test_10_New creates a new service from an optional name.
func (test *TestServiceSuite) Test_10_New() {
	s := test.Suite.Require

	// Creating a service without a name uses the default name.
	independent, err := New()
	s().NoError(err)
	s().Equal(DefaultName, independent.Name())
	s().NoError(independent.topologyHandler.Close())

	// Wait a bit for closing context threads
	time.Sleep(time.Millisecond * 100)

	independent, err = New(test.name)
	s().NoError(err)
	s().Equal(test.name, independent.Name())

	// remove the created service.
	// to re-create the service, we must close the context.
	s().NoError(independent.topologyHandler.Close())
	// wait a bit for closing context threads
	time.Sleep(time.Millisecond * 500)

}

// Test_14_manager tests the creation of the manager and linting it with the handler.
func (test *TestServiceSuite) Test_14_manager() {
	s := test.Suite.Require

	test.newService()

	s().NoError(test.service.newManager())

	handler := test.service.Handlers["main"].(base.Interface)
	err := test.service.setHandlerClient(handler)
	s().NoError(err)

	test.closeService()
}

// Test_15_handler tests setup and start of the handler
func (test *TestServiceSuite) Test_15_handler() {
	s := test.Suite.Require

	test.newService()

	s().NoError(test.service.newManager())

	handler := test.mainHandler()
	s().NoError(test.service.startHandler(handler))

	// wait a bit until the handler is initialized
	time.Sleep(time.Millisecond * 100)

	// let's test that handler runs
	externalClient := test.externalClient(handler.Config())

	// request the handler
	req := message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	}
	reply, err := externalClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// close the handler
	handlerManager, err := manager_client.New(handler.Config())
	s().NoError(err)
	s().NoError(handlerManager.Close())
	s().NoError(externalClient.Close())

	test.closeService()
}

// Test_16_managerRequest tests the start of the manager and closing it by a command
func (test *TestServiceSuite) Test_16_managerRequest() {
	s := test.Suite.Require

	test.newService()

	s().NoError(test.service.newManager())

	handler := test.service.Handlers["main"].(base.Interface)
	err := test.service.setHandlerClient(handler)
	s().NoError(err)

	s().NoError(test.service.startHandler(handler))

	s().NoError(test.service.manager.Start())

	// wait a bit until the handler and manager are initialized
	time.Sleep(time.Millisecond * 100)
	s().True(test.service.manager.Running())

	// test sending a command to the manager
	createdConfig, err := test.service.topologyHandler.Config().Service(test.name)
	s().NoError(err)
	externalConfig := createdConfig.Manager
	externalConfig.UrlFunc(clientConfig.Url)
	externalClient, err := client.New(externalConfig)
	s().NoError(err)

	req := message.Request{
		Command:    "close",
		Parameters: datatype.New(),
	}
	err = externalClient.Submit(&req)
	s().NoError(err)

	// Wait a bit for closing service threads
	time.Sleep(time.Millisecond * 100)

	// make sure that context is not running
	s().False(test.service.topologyHandler.IsRunning())
	s().False(test.service.manager.Running())

	// clean out
	test.service = nil
}

// Test_17_Start test service start.
// It's the collection of all previous tested functions together
// The started service will make the handler and managers available
func (test *TestServiceSuite) Test_17_Start() {
	s := test.Require

	test.newService()

	err := test.service.Start()
	s().NoError(err)

	// wait a bit for thread initialization
	time.Sleep(time.Millisecond * 100)

	// let's test that handler runs
	mainHandler := test.mainHandler()
	externalClient := test.externalClient(mainHandler.Config())

	// Make sure that handlers are running
	req := message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	}
	reply, err := externalClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// Make sure that manager is running
	managerClient := test.managerClient()
	req = message.Request{
		Command:    "heartbeat",
		Parameters: datatype.New(),
	}
	reply, err = managerClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// clean out
	// we don't close the handler here by calling mainHandler.Close.
	//
	// the service manager must close all handlers.
	s().NoError(test.service.manager.StopService(test.service.Name()))

	// since we closed by manager, the cleaning-out by test suite not necessary.
	test.service = nil
}

// Test_22_Start_Close test service start then close in repeat.
// It's the collection of all previous tested functions together
// The started service will make the handler and managers available
func (test *TestServiceSuite) Test_22_Start_Close() {
	s := test.Require

	test.newService()

	err := test.service.Start()
	s().NoError(err)

	// wait a bit for thread initialization
	time.Sleep(time.Millisecond * 100)

	// let's test that handler runs
	mainHandler := test.mainHandler()
	externalClient := test.externalClient(mainHandler.Config())

	// Make sure that handlers are running
	req := message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	}
	reply, err := externalClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// Make sure that manager is running
	managerClient := test.managerClient()
	req = message.Request{
		Command:    "heartbeat",
		Parameters: datatype.New(),
	}
	reply, err = managerClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// clean out
	// we don't close the handler here by calling mainHandler.Close.
	//
	// the service manager must close all handlers.
	s().NoError(test.service.manager.StopService(test.service.Name()))
	time.Sleep(time.Millisecond * 100)

	// since we closed by manager, the cleaning-out by test suite not necessary.
	test.service = nil

	//
	// Repeat starting the service again
	//

	test.newService()
	err = test.service.Start()
	s().NoError(err)

	// wait a bit for thread initialization
	time.Sleep(time.Millisecond * 100)

	// let's test that handler runs
	mainHandler = test.mainHandler()
	externalClient = test.externalClient(mainHandler.Config())

	// Make sure that handlers are running
	req = message.Request{
		Command:    "hello",
		Parameters: datatype.New(),
	}
	reply, err = externalClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// Make sure that manager is running
	managerClient = test.managerClient()
	req = message.Request{
		Command:    "heartbeat",
		Parameters: datatype.New(),
	}
	reply, err = managerClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// clean out
	// we don't close the handler here by calling mainHandler.Close.
	//
	// the service manager must close all handlers.
	s().NoError(test.service.manager.StopService(test.service.Name()))

	// since we closed by manager, the cleaning-out by test suite not necessary.
	test.service = nil
	time.Sleep(time.Millisecond * 100)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestService(t *testing.T) {
	suite.Run(t, new(TestServiceSuite))
}
