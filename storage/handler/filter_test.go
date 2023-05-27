package handler

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/storage/configuration"
	"github.com/blocklords/sds/storage/smartcontract"
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
	sm        smartcontract.Smartcontract
	conf_list *key_value.List
	sm_list   *key_value.List
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

	sm_0 := smartcontract.Smartcontract{
		SmartcontractKey: smartcontract_key.Key{
			NetworkId: "test_1",
			Address:   "0xaddr_0",
		},
		AbiId: "abi",
	}
	suite.sm = sm_0

	sm_1 := smartcontract.Smartcontract{
		SmartcontractKey: smartcontract_key.Key{
			NetworkId: "test_1",
			Address:   "0xaddr_1",
		},
		AbiId: "abi",
	}

	sm_2 := smartcontract.Smartcontract{
		SmartcontractKey: smartcontract_key.Key{
			NetworkId: "test_2",
			Address:   "0xaddr_2",
		},
		AbiId: "abi",
	}

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

	list := key_value.NewList()
	err = list.Add(conf_0.Topic, &conf_0)
	suite.Require().NoError(err)

	err = list.Add(conf_1.Topic, &conf_1)
	suite.Require().NoError(err)
	suite.conf_list = list

	err = list.Add(conf_2.Topic, &conf_2)
	suite.Require().NoError(err)
	suite.conf_list = list

	sm_list := key_value.NewList()
	err = sm_list.Add(sm_0.SmartcontractKey, &sm_0)
	suite.Require().NoError(err)

	err = sm_list.Add(sm_1.SmartcontractKey, &sm_1)
	suite.Require().NoError(err)

	err = sm_list.Add(sm_2.SmartcontractKey, &sm_2)
	suite.Require().NoError(err)

	suite.sm_list = sm_list
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

func (suite *TestFilterSuite) TestConfigurationFiltering() {
	topic_filter := topic.TopicFilter{
		Organizations: []string{"test_org"},
		NetworkIds:    []string{"test_1"},
	}
	new_list := filter_configuration(suite.conf_list, &topic_filter)
	suite.Require().Len(new_list, 1)
}

func (suite *TestFilterSuite) TestSmartcontractFiltering() {
	topic_filter := topic.TopicFilter{
		Organizations: []string{"test_org"},
		NetworkIds:    []string{"test_1"},
	}
	new_list := filter_configuration(suite.conf_list, &topic_filter)

	suite.T().Log("configs", new_list[0])

	filtered_sm, filtered_topics, err := filter_smartcontract(new_list, suite.sm_list)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(filtered_sm)
	suite.Require().EqualValues(suite.conf.Topic.ToString(topic.SMARTCONTRACT_LEVEL), filtered_topics[0])
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFilter(t *testing.T) {
	suite.Run(t, new(TestFilterSuite))
}
