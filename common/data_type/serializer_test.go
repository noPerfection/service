package data_type

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSerializerSuite struct {
	suite.Suite
}

// Setup
// Setup checks the New() functions
// Setup checks ToMap() functions
func (suite *TestSerializerSuite) SetupTest() {
}

func (suite *TestSerializerSuite) TestSerialization() {
	type Item struct {
		Param1 string `json:"param_1"`
		Param2 uint64 `json:"param_2"`
	}
	sample := Item{Param1: "hello", Param2: uint64(5)}

	body, err := Serialize(sample)
	suite.Require().NoError(err)

	expected := `{"param_1":"hello","param_2":5}`
	suite.Require().EqualValues(expected, string(body))

	var new_sample Item
	err = Deserialize(body, &new_sample)
	suite.Require().NoError(err)
	suite.Require().EqualValues(new_sample, sample)

	// try to serialize without passing the reference
	var no_ref Item
	err = Deserialize(body, no_ref)
	suite.Require().Error(err)
	suite.Require().Empty(no_ref)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSerializer(t *testing.T) {
	suite.Run(t, new(TestSerializerSuite))
}
