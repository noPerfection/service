package provider

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
type TestProviderSuite struct {
	suite.Suite
	provider Provider
}

func (suite *TestProviderSuite) SetupTest() {
	suite.provider = Provider{
		Url:    "https://sample.com",
		Length: 32,
	}
}

func (suite *TestProviderSuite) TestNew() {
	// empty map key should fail
	// as transaction.validate() will fail
	kv := key_value.Empty()
	_, err := New(kv)
	suite.Require().Error(err)

	// one of the parameters is missing
	// here its missing to have "length"
	kv = key_value.Empty().
		Set("url", "https://sample.com")
	_, err = New(kv)
	suite.Require().Error(err)

	// one of the parameters is missing
	// here its missing to have "url"
	kv = key_value.Empty().
		Set("length", uint64(32))
	_, err = New(kv)
	suite.Require().Error(err)

	// the url is empty
	kv = key_value.Empty().
		Set("url", "").
		Set("length", uint64(32))
	_, err = New(kv)
	suite.Require().Error(err)

	// the length is not a valid a number
	kv = key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", "should_be_number")
	_, err = New(kv)
	suite.Require().Error(err)

	// the length is not a uint64
	// but its find, because key_value.KeyValue
	// will convert it to json.Number
	kv = key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", int(32))
	_, err = New(kv)
	suite.Require().NoError(err)

	// the length should not be 0
	kv = key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", uint64(0))
	_, err = New(kv)
	suite.Require().Error(err)

	// the length should not exceed 10_000
	kv = key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", uint64(10_001))
	_, err = New(kv)
	suite.Require().Error(err)

	// the protocol is either https or http
	kv = key_value.Empty().
		Set("url", "ws://sample.com").
		Set("length", uint64(10))
	_, err = New(kv)
	suite.Require().Error(err)

	// invalid url
	kv = key_value.Empty().
		Set("url", "http://item asdsa").
		Set("length", uint64(10))
	_, err = New(kv)
	suite.Require().Error(err)

	// the correct provider
	kv = key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", uint64(32))
	provider, err := New(kv)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.provider, provider)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestProvider(t *testing.T) {
	suite.Run(t, new(TestProviderSuite))
}
