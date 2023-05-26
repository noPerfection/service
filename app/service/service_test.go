package service

import (
	"testing"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestServiceSuite struct {
	suite.Suite
	inproc_service     *Service
	this_service       *Service
	remote_service     *Service
	broadcast_service  *Service
	subscriber_service *Service
}

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestServiceSuite) SetupTest() {
	// Create the inprocess url
	_, err := Inprocess("sadsad")
	suite.Require().Error(err)

	inproc, err := Inprocess("BLOCKCHAIN")
	suite.Require().NoError(err)
	suite.inproc_service = inproc

	suite.Require().Equal("inproc://SERVICE_BLOCKCHAIN", inproc.Url())
	suite.Require().Equal("BLOCKCHAIN", inproc.Name)
	suite.Require().True(inproc.IsInproc())

	////////////////////////////////////////////////
	//
	// Create the external url
	//
	////////////////////////////////////////////////
	logger, err := log.New("test-suite", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)
	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)

	// the service type is invalid.
	_, err = NewExternal("sadsad", THIS, app_config)
	suite.Require().Error(err)

	// the limit is
	_, err = NewExternal(CATEGORIZER, Limit(5), app_config)
	suite.Require().Error(err)

	// the app config is empty
	_, err = NewExternal(CATEGORIZER, THIS, nil)
	suite.Require().Error(err)

	service, err := NewExternal(CATEGORIZER, THIS, app_config)
	suite.Require().NoError(err)
	suite.this_service = service

	service, err = NewExternal(CATEGORIZER, SUBSCRIBE, app_config)
	suite.Require().NoError(err)
	suite.subscriber_service = service

	service, err = NewExternal(CATEGORIZER, BROADCAST, app_config)
	suite.Require().NoError(err)
	suite.broadcast_service = service

	service, err = NewExternal(CATEGORIZER, REMOTE, app_config)
	suite.Require().NoError(err)
	suite.remote_service = service
}

func (suite *TestServiceSuite) TestValidation() {
	suite.Require().Equal("inproc://SERVICE_BLOCKCHAIN", suite.inproc_service.Url())
	suite.Require().True(suite.inproc_service.IsInproc())
	suite.Require().False(suite.inproc_service.IsBroadcast())
	suite.Require().False(suite.inproc_service.IsSubscribe())
	suite.Require().False(suite.inproc_service.IsRemote())
	suite.Require().False(suite.inproc_service.IsThis())

	suite.Require().Equal("tcp://*:4020", suite.this_service.Url())
	suite.Require().False(suite.this_service.IsInproc())
	suite.Require().False(suite.this_service.IsBroadcast())
	suite.Require().False(suite.this_service.IsSubscribe())
	suite.Require().False(suite.this_service.IsRemote())
	suite.Require().True(suite.this_service.IsThis())

	suite.Require().Equal("tcp://localhost:4020", suite.remote_service.Url())
	suite.Require().False(suite.remote_service.IsInproc())
	suite.Require().False(suite.remote_service.IsBroadcast())
	suite.Require().False(suite.remote_service.IsSubscribe())
	suite.Require().True(suite.remote_service.IsRemote())
	suite.Require().False(suite.remote_service.IsThis())

	suite.Require().Equal("tcp://*:4021", suite.broadcast_service.Url())
	suite.Require().False(suite.broadcast_service.IsInproc())
	suite.Require().True(suite.broadcast_service.IsBroadcast())
	suite.Require().False(suite.broadcast_service.IsSubscribe())
	suite.Require().False(suite.broadcast_service.IsRemote())
	suite.Require().False(suite.broadcast_service.IsThis())

	suite.Require().Equal("tcp://localhost:4021", suite.subscriber_service.Url())
	suite.Require().False(suite.subscriber_service.IsInproc())
	suite.Require().False(suite.subscriber_service.IsBroadcast())
	suite.Require().True(suite.subscriber_service.IsSubscribe())
	suite.Require().False(suite.subscriber_service.IsRemote())
	suite.Require().False(suite.subscriber_service.IsThis())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestService(t *testing.T) {
	suite.Run(t, new(TestServiceSuite))
}
