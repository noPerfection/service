package configuration

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestConfigurationSuite struct {
	suite.Suite
	configuration Configuration
}

func (suite *TestConfigurationSuite) SetupTest() {
	address := "0xaddress"
	sample := topic.Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
	}

	suite.configuration = Configuration{
		Topic:   sample,
		Address: address,
	}
}

func (suite *TestConfigurationSuite) TestNew() {
	// creating a new smartcontract from empty parameter
	// should fail
	kv := key_value.Empty()
	_, err := New(kv)
	suite.Require().Error(err)

	// topic is not on smartcontract level
	address := "0xaddress"
	sample := topic.Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
		Event:         "Transfer",
	}
	kv = key_value.Empty().
		Set("topic", sample).
		Set("address", address)
	_, err = New(kv)
	suite.Require().Error(err)

	// topic is not valid no level
	sample = topic.Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Smartcontract: "TestErc20",
	}
	kv = key_value.Empty().
		Set("topic", sample).
		Set("address", address)
	_, err = New(kv)
	suite.Require().Error(err)

	// topic is not a smartcontract level
	sample = topic.Topic{
		Organization: "seascape",
		Project:      "sds-core",
		NetworkId:    "1",
		Group:        "test-suite",
	}
	kv = key_value.Empty().
		Set("topic", sample).
		Set("address", address)
	_, err = New(kv)
	suite.Require().Error(err)

	// invalid address
	sample = topic.Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
	}
	kv = key_value.Empty().
		Set("topic", sample).
		Set("address", "")
	_, err = New(kv)
	suite.Require().Error(err)

	topic_conf, err := NewFromTopic(sample, address)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.configuration, *topic_conf)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestConfiguration(t *testing.T) {
	suite.Run(t, new(TestConfigurationSuite))
}
