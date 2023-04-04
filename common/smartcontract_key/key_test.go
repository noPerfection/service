package smartcontract_key

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestKeySuite struct {
	suite.Suite
	key Key
}

func (suite *TestKeySuite) SetupTest() {
	network_id := "1"
	address := "0x123"
	uint_map := key_value.Empty().
		Set("network_id", network_id).
		Set("address", address)

	key, _ := New(network_id, address)
	suite.Require().Equal(network_id, key.NetworkId)
	suite.Require().Equal(address, key.Address)

	map_key, err := NewFromKeyValue(uint_map)
	suite.Require().NoError(err)
	suite.Require().Equal(network_id, map_key.NetworkId)
	suite.Require().Equal(address, map_key.Address)

	uint_map = key_value.Empty().
		Set("address", address)
	_, err = NewFromKeyValue(uint_map)
	suite.Require().Error(err)

	uint_map = key_value.Empty().
		Set("network_id", network_id)
	_, err = NewFromKeyValue(uint_map)
	suite.Require().Error(err)

	uint_map = key_value.Empty()
	_, err = NewFromKeyValue(uint_map)
	suite.Require().Error(err)

	uint_map = key_value.Empty().
		Set("network_id", network_id).
		Set("address", address).
		Set("additional_param", uint64(1))
	_, err = NewFromKeyValue(uint_map)
	suite.Require().NoError(err)

	suite.key = key
}

func (suite *TestKeySuite) TestToString() {
	key_string := "1.0x123"
	suite.Require().Equal(key_string, suite.key.ToString())

	key, err := NewFromString(key_string)
	suite.Require().NoError(err)
	suite.Require().Equal(suite.key, key)

	no_network_string := ".0x123"
	_, err = NewFromString(no_network_string)
	suite.Require().Error(err)

	no_address := "1."
	_, err = NewFromString(no_address)
	suite.Require().Error(err)

	no_parameters := "."
	_, err = NewFromString(no_parameters)
	suite.Require().Error(err)

	empty := ""
	_, err = NewFromString(empty)
	suite.Require().Error(err)

	too_many_parameters := "1.0x1232.123213"
	_, err = NewFromString(too_many_parameters)
	suite.Require().Error(err)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockHeader(t *testing.T) {
	suite.Run(t, new(TestKeySuite))
}
