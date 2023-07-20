package identity

import (
	"testing"

	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/log"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestServiceSuite struct {
	suite.Suite
	inprocService     *Service
	thisService       *Service
	remoteService     *Service
	broadcastService  *Service
	subscriberService *Service
}

// Todo test in-process and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestServiceSuite) SetupTest() {
	// Create the inprocess url
	_, err := Inprocess("does-not-exist")
	suite.Require().Error(err)

	inproc, err := Inprocess("BLOCKCHAIN")
	suite.Require().NoError(err)
	suite.inprocService = inproc

	suite.Require().Equal("inproc://SERVICE_BLOCKCHAIN", inproc.Url())
	suite.Require().Equal("BLOCKCHAIN", inproc.Name)
	suite.Require().True(inproc.IsInproc())

	////////////////////////////////////////////////
	//
	// Create the external url
	//
	////////////////////////////////////////////////
	logger, err := log.New("test-suite", true)
	suite.Require().NoError(err)
	appConfig, err := configuration.New(logger)
	suite.Require().NoError(err)

	// the service type is invalid.
	_, err = NewExternal("does-not-exist", THIS, appConfig)
	suite.Require().Error(err)

	// the limit is
	_, err = NewExternal("indexer", Limit(5), appConfig)
	suite.Require().Error(err)

	// the app config is empty
	_, err = NewExternal("INDEXER", THIS, nil)
	suite.Require().Error(err)

	service, err := NewExternal("INDEXER", THIS, appConfig)
	suite.Require().NoError(err)
	suite.thisService = service

	service, err = NewExternal("INDEXER", SUBSCRIBE, appConfig)
	suite.Require().NoError(err)
	suite.subscriberService = service

	service, err = NewExternal("INDEXER", BROADCAST, appConfig)
	suite.Require().NoError(err)
	suite.broadcastService = service

	service, err = NewExternal("INDEXER", REMOTE, appConfig)
	suite.Require().NoError(err)
	suite.remoteService = service
}

func (suite *TestServiceSuite) TestValidation() {
	suite.Require().Equal("inproc://SERVICE_BLOCKCHAIN", suite.inprocService.Url())
	suite.Require().True(suite.inprocService.IsInproc())
	suite.Require().False(suite.inprocService.IsBroadcast())
	suite.Require().False(suite.inprocService.IsSubscribe())
	suite.Require().False(suite.inprocService.IsRemote())
	suite.Require().False(suite.inprocService.IsThis())

	suite.Require().Equal("tcp://*:4020", suite.thisService.Url())
	suite.Require().False(suite.thisService.IsInproc())
	suite.Require().False(suite.thisService.IsBroadcast())
	suite.Require().False(suite.thisService.IsSubscribe())
	suite.Require().False(suite.thisService.IsRemote())
	suite.Require().True(suite.thisService.IsThis())

	suite.Require().Equal("tcp://localhost:4020", suite.remoteService.Url())
	suite.Require().False(suite.remoteService.IsInproc())
	suite.Require().False(suite.remoteService.IsBroadcast())
	suite.Require().False(suite.remoteService.IsSubscribe())
	suite.Require().True(suite.remoteService.IsRemote())
	suite.Require().False(suite.remoteService.IsThis())

	suite.Require().Equal("tcp://*:4021", suite.broadcastService.Url())
	suite.Require().False(suite.broadcastService.IsInproc())
	suite.Require().True(suite.broadcastService.IsBroadcast())
	suite.Require().False(suite.broadcastService.IsSubscribe())
	suite.Require().False(suite.broadcastService.IsRemote())
	suite.Require().False(suite.broadcastService.IsThis())

	suite.Require().Equal("tcp://localhost:4021", suite.subscriberService.Url())
	suite.Require().False(suite.subscriberService.IsInproc())
	suite.Require().False(suite.subscriberService.IsBroadcast())
	suite.Require().True(suite.subscriberService.IsSubscribe())
	suite.Require().False(suite.subscriberService.IsRemote())
	suite.Require().False(suite.subscriberService.IsThis())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestService(t *testing.T) {
	suite.Run(t, new(TestServiceSuite))
}
