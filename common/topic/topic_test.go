package topic

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Test creation
//   - from parameters
//   - from json
//   - from string
//     topic filter string to topic string
//     should fail
//
// compare the level (for each nesting) against constants
//
// Test the string creation
// for each level
type TestTopicSuite struct {
	suite.Suite
	topic        Topic
	topic_string TopicString
}

// Setup
// Setup checks the New() functions
// Setup checks ToMap() functions
func (suite *TestTopicSuite) SetupTest() {
	sample := Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
		Event:         "Transfer",
	}
	topic_string := AsTopicString(`o:seascape;p:sds-core;n:1;g:test-suite;s:TestErc20;e:Transfer`)

	suite.topic = sample
	suite.topic_string = topic_string

	suite.Require().Equal(topic_string, sample.ToString(FULL_LEVEL))
}

func (suite *TestTopicSuite) TestStringParse() {
	new_topic, err := ParseString(suite.topic_string)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.topic, new_topic)

	// additional parameter in the topic string should fail
	topic_string := AsTopicString(`o:seascape;p:sds-core;n:1;g:test-suite;s:TestErc20;e:Transfer;m:transfer`)
	_, err = ParseString(topic_string)
	suite.Require().Error(err)

	// case sensitive
	topic_string = AsTopicString(`O:seascape;p:sds-core;n:1;g:test-suite;s:TestErc20;e:Transfer`)
	_, err = ParseString(topic_string)
	suite.Require().Error(err)

	// additional semicolon should fail
	topic_string = AsTopicString(`o:seascape;p:sds-core;n:1;g:test-suite;s:TestErc20;e:Transfer;`)
	_, err = ParseString(topic_string)
	suite.Require().Error(err)

	// missing the one of the paths
	// if the event is given, then all previous levels
	// should be given too.
	// missing "network_id"
	topic_string = AsTopicString(`o:seascape;p:sds-core;g:test-suite;s:TestErc20;e:Transfer`)
	_, err = ParseString(topic_string)
	suite.Require().Error(err)

	// value of the topic path is not a literal
	// it has not required tokens.
	topic_string = AsTopicString(`o:seascape:network;p:sds-core;n:1;g:test-suite;s:TestErc20;e:Transfer`)
	_, err = ParseString(topic_string)
	suite.Require().Error(err)
}

func (suite *TestTopicSuite) TestParsingJson() {
	kv := key_value.Empty().
		Set("o", "seascape").
		Set("p", "sds-core").
		Set("n", "1").
		Set("g", "test-suite").
		Set("s", "TestErc20").
		Set("e", "Transfer")

	new_topic, err := ParseJSON(kv)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.topic, *new_topic)

	// changing the orders doesn't affect the topic
	kv = key_value.Empty().
		Set("o", "seascape").
		Set("n", "1").
		Set("p", "sds-core").
		Set("g", "test-suite").
		Set("s", "TestErc20").
		Set("e", "Transfer")

	new_topic, err = ParseJSON(kv)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.topic, *new_topic)

	// additional parameter in the topic string
	// should succeed, but the value will be missed
	kv.Set("m", "transfer")
	_, err = ParseJSON(kv)
	suite.Require().NoError(err)

	// setting with the empty parameter should fail
	// empty group
	invalid_kv := key_value.Empty().
		Set("o", "seascape").
		Set("p", "sds").
		Set("n", "1").
		Set("g", "").
		Set("s", "TestErc20").
		Set("e", "Transfer")
	_, err = ParseJSON(invalid_kv)
	suite.Require().Error(err)

	// case sensitive
	// Group name is given as 'G', should be 'g'
	invalid_kv = key_value.Empty().
		Set("o", "seascape").
		Set("p", "sds").
		Set("n", "1").
		Set("G", "test-suite").
		Set("s", "TestErc20").
		Set("e", "Transfer")
	_, err = ParseJSON(invalid_kv)
	suite.Require().Error(err)

	// missing the one of the paths
	// if the event is given, then all previous levels
	// should be given too.
	// missing "group"
	invalid_kv = key_value.Empty().
		Set("o", "seascape").
		Set("p", "sds").
		Set("n", "1").
		Set("s", "TestErc20").
		Set("e", "Transfer")
	_, err = ParseJSON(invalid_kv)
	suite.Require().Error(err)
}

func (suite *TestTopicSuite) TestToString() {
	topic := Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
		Event:         "Transfer",
	}

	topic_string := topic.ToString(0)
	suite.Require().Empty(topic_string)

	topic_string = topic.ToString(7)
	suite.Require().Empty(topic_string)

	expected_topic_string := TopicString(`o:seascape;p:sds-core;n:1;g:test-suite;s:TestErc20;e:Transfer`)
	topic_string = topic.ToString(6)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	expected_topic_string = TopicString(`o:seascape;p:sds-core;n:1;g:test-suite;s:TestErc20`)
	topic_string = topic.ToString(5)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	expected_topic_string = TopicString(`o:seascape;p:sds-core;n:1;g:test-suite`)
	topic_string = topic.ToString(4)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	expected_topic_string = TopicString(`o:seascape;p:sds-core;n:1`)
	topic_string = topic.ToString(3)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	expected_topic_string = TopicString(`o:seascape;p:sds-core`)
	topic_string = topic.ToString(2)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	expected_topic_string = TopicString(`o:seascape`)
	topic_string = topic.ToString(1)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	expected_topic_string = TopicString(`o:seascape`)
	suite.Require().EqualValues(expected_topic_string, topic_string)

	topic = Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
		Event:         "Transfer",
	}
	topic_string = topic.ToString(FULL_LEVEL)
	suite.Require().Empty(topic_string)

	// NetworkId is empty, the upper root exists
	// But all topic should be valid
	topic_string = topic.ToString(PROJECT_LEVEL)
	suite.Require().Empty(topic_string)

	topic = Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
		Event:         "Transfer",
	}
	topic_string = topic.ToString(FULL_LEVEL)
	suite.Require().Empty(topic_string)

	topic = Topic{
		Organization:  "seascape",
		Project:       "sds-core",
		NetworkId:     "1",
		Group:         "test-suite",
		Smartcontract: "TestErc20",
	}
	// the topic is FULL_LEVEL
	// but we try to get full path
	// it should fail
	topic_string = topic.ToString(FULL_LEVEL)
	suite.Require().Empty(topic_string)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestTopic(t *testing.T) {
	suite.Run(t, new(TestTopicSuite))
}
