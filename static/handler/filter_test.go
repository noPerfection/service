package handler

import (
	"testing"

	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/static/configuration"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestFilterSuite struct {
	suite.Suite
	logger    log.Logger
	conf      configuration.Configuration
	conf_list *key_value.List
}

/*
Two organization

	first one has 1 conf
	second one has many

	org has 2 orgs
*/
func (suite *TestFilterSuite) SetupTest() {
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
		Address: "0xaddr_0",
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
		Address: "0xaddr_1",
	}

	conf_2 := configuration.Configuration{
		Topic: topic.Topic{
			Organization:  "test_org",
			Project:       "test_proj_2",
			NetworkId:     "test_2",
			Group:         "test_group_2",
			Smartcontract: "test_name_2",
		},
		Address: "0xaddr_2",
	}
	suite.conf = conf_0

	list := key_value.NewList()
	err = list.Add(conf_0.Topic, &conf_0)
	suite.Require().NoError(err)

	err = list.Add(conf_1.Topic, &conf_1)
	suite.Require().NoError(err)
	suite.conf_list = list

	err = list.Add(conf_2.Topic, &conf_2)
	suite.Require().NoError(err)
	suite.conf_list = list
}

func (suite *TestFilterSuite) TestOrganizationFilter() {
	// empty paths should return all configurations
	paths := []string{}
	new_list := filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 3)
	suite.Require().False(new_list.IsEmpty())

	// fetching the non existing paths should return empty list
	paths = []string{"no_org"}
	new_list = filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 0)
	suite.Require().True(new_list.IsEmpty())

	// fetching the org that has one element
	paths = []string{"test_org_1"}
	new_list = filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 1)
	suite.Require().False(new_list.IsEmpty())

	// fetching the org that has two element
	paths = []string{"test_org"}
	new_list = filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 2)
	suite.Require().False(new_list.IsEmpty())

	paths = []string{"test_org", "test_org_1"}
	new_list = filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 3)
	suite.Require().False(new_list.IsEmpty())

	paths = []string{"test_org", "test_org_1", "non_exist"}
	new_list = filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 3)
	suite.Require().False(new_list.IsEmpty())
}

func (suite *TestFilterSuite) TestNetworkIdFilter() {
	// empty paths should return all configurations
	paths := []string{}
	new_list := filter_network_id(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 3)
	suite.Require().False(new_list.IsEmpty())

	// fetching the non existing paths should return empty list
	paths = []string{"ideal_blockchain"}
	new_list = filter_network_id(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 0)
	suite.Require().True(new_list.IsEmpty())

	// fetching the org that has one element
	paths = []string{"test_2"}
	new_list = filter_network_id(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 1)
	suite.Require().False(new_list.IsEmpty())

	// fetching the org that has two element
	paths = []string{"test_1"}
	new_list = filter_network_id(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 2)
	suite.Require().False(new_list.IsEmpty())

	paths = []string{"test_1", "test_2"}
	new_list = filter_network_id(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 3)
	suite.Require().False(new_list.IsEmpty())

	paths = []string{"test_org"}
	new_list = filter_organization(suite.conf_list, paths)
	suite.Require().Equal(new_list.Len(), 2)
	suite.Require().False(new_list.IsEmpty())

	// fetching from new list should be successful
	paths = []string{"test_1"}
	new_list = filter_network_id(new_list, paths)
	suite.Require().Equal(new_list.Len(), 1)
	suite.Require().False(new_list.IsEmpty())
}

func (suite *TestFilterSuite) TestGet() {
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

func (suite *TestFilterSuite) TestSet() {
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
func TestFilter(t *testing.T) {
	suite.Run(t, new(TestFilterSuite))
}
