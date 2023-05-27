package handler

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/service/communication/message"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/storage/configuration"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestConfigurationSuite struct {
	suite.Suite
	logger    log.Logger
	conf      configuration.Configuration
	conf_list *key_value.List
}

func (suite *TestConfigurationSuite) SetupTest() {
	logger, err := log.New("test", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)
	suite.logger = logger

	conf_0 := configuration.Configuration{
		Topic: topic.Topic{
			Organization:  "test_org",
			Project:       "test_proj",
			NetworkId:     "test_1",
			Group:         "test_group",
			Smartcontract: "test_name",
		},
		Address: "0xaddress",
	}
	suite.conf = conf_0

	conf_1 := configuration.Configuration{
		Topic: topic.Topic{
			Organization:  "test_org_1",
			Project:       "test_proj_1",
			NetworkId:     "test_1",
			Group:         "test_group_1",
			Smartcontract: "test_name_1",
		},
		Address: "0xaddress",
	}

	list := key_value.NewList()
	err = list.Add(conf_0.Topic, &conf_0)
	suite.Require().NoError(err)

	err = list.Add(conf_1.Topic, &conf_1)
	suite.Require().NoError(err)
	suite.conf_list = list
}

func (suite *TestConfigurationSuite) TestGet() {
	// valid request
	valid_kv, err := key_value.NewFromInterface(suite.conf.Topic)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply := ConfigurationGet(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.Require().True(reply.IsOK())

	var replied_sm GetConfigurationReply
	err = reply.Parameters.ToInterface(&replied_sm)
	suite.Require().NoError(err)

	suite.Require().EqualValues(suite.conf, replied_sm)

	// request with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = ConfigurationGet(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.Require().False(reply.IsOK())

	// request of configuration that
	// doesn't exist in the list
	// should fail
	no_topic := topic.Topic{
		Organization:  "test_org_2",
		Project:       "test_proj_2",
		NetworkId:     "test_1",
		Group:         "test_group_2",
		Smartcontract: "test_name_2",
	}
	topic_kv, err := key_value.NewFromInterface(no_topic)
	suite.Require().NoError(err)

	request = message.Request{
		Command:    "",
		Parameters: topic_kv,
	}
	reply = ConfigurationGet(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.Require().False(reply.IsOK())

	// requesting with invalid type for abi id should fail
	no_topic = topic.Topic{
		Organization: "test_org_2",
		Project:      "test_proj_2",
		NetworkId:    "test_1",
		Group:        "test_group_2",
	}
	topic_kv, err = key_value.NewFromInterface(no_topic)
	suite.Require().NoError(err)
	request = message.Request{
		Command:    "",
		Parameters: topic_kv,
	}
	reply = ConfigurationGet(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.Require().False(reply.IsOK())
}

func (suite *TestConfigurationSuite) TestSet() {
	// valid request
	no_topic := topic.Topic{
		Organization:  "test_org_2",
		Project:       "test_proj_2",
		NetworkId:     "test_1",
		Group:         "test_group_2",
		Smartcontract: "test_name_2",
	}
	valid_request := configuration.Configuration{
		Topic:   no_topic,
		Address: "0xaddress_3",
	}
	valid_kv, err := key_value.NewFromInterface(valid_request)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply := ConfigurationRegister(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.T().Log(reply.Message)
	suite.Require().True(reply.IsOK())

	var replied_sm GetConfigurationReply
	err = reply.Parameters.ToInterface(&replied_sm)
	suite.Require().NoError(err)
	suite.Require().EqualValues(valid_request, replied_sm)

	// the abi list should have the item
	sm_in_list, err := suite.conf_list.Get(replied_sm.Topic)
	suite.Require().NoError(err)
	suite.Require().EqualValues(&replied_sm, sm_in_list)

	// registering with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = ConfigurationRegister(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.Require().False(reply.IsOK())

	// registering of abi that already exist in the list
	// should fail
	request = message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply = ConfigurationRegister(request, suite.logger, nil, nil, nil, suite.conf_list)
	suite.Require().False(reply.IsOK())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestConfiguration(t *testing.T) {
	suite.Run(t, new(TestConfigurationSuite))
}
