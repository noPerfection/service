package network

import (
	"testing"

	"github.com/blocklords/sds/blockchain/network/provider"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/configuration"
	"github.com/blocklords/sds/service/log"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestNetworkSuite struct {
	suite.Suite
	network Network
}

func (suite *TestNetworkSuite) SetupTest() {
	provider_1 := provider.Provider{
		Url:    "https://sample.com",
		Length: 3,
	}
	provider_2 := provider.Provider{
		Url:    "https://example.com",
		Length: 32,
	}
	network_id := "1"
	network_type := EVM

	suite.network = Network{
		Providers: []provider.Provider{
			provider_1,
			provider_2,
		},
		Id:   network_id,
		Type: network_type,
	}

}

func (suite *TestNetworkSuite) TestFirstProvider() {
	expected_url := "https://sample.com"
	expected_length := uint64(3)

	actual_url, err := suite.network.GetFirstProviderUrl()
	suite.Require().NoError(err)
	suite.Require().Equal(expected_url, actual_url)

	actual_length, err := suite.network.GetFirstProviderLength()
	suite.Require().NoError(err)
	suite.Require().Equal(expected_length, actual_length)

	// Empty network should return error
	network := Network{}
	_, err = network.GetFirstProviderLength()
	suite.Require().Error(err)
	_, err = network.GetFirstProviderUrl()
	suite.Require().Error(err)
}

func (suite *TestNetworkSuite) TestNetworkType() {
	evm := "evm"
	evm_type, err := NewNetworkType(evm)
	suite.Require().NoError(err)
	suite.Require().Equal(EVM, evm_type)
	suite.Require().Equal(evm, evm_type.String())

	// the unsupported network type
	ethereum := "ethereum"
	_, err = NewNetworkType(ethereum)
	suite.Require().Error(err)
}

func (suite *TestNetworkSuite) TestNew() {
	// empty map key should fail
	kv := key_value.Empty()
	_, err := New(kv)
	suite.Require().Error(err)

	// one of the parameters in the provider is missing
	// here its missing to have "length"
	provider_kv := key_value.Empty().
		Set("url", "https://sample.com")
	kv = key_value.Empty().
		Set("id", "yes").
		Set("type", "evm").
		Set("providers", []key_value.KeyValue{provider_kv})
	_, err = New(kv)
	suite.Require().Error(err)

	// network id should be string only
	provider_kv = key_value.Empty().
		Set("length", 32).
		Set("url", "https://sample.com")
	kv = key_value.Empty().
		Set("id", 4).
		Set("type", "evm").
		Set("providers", []key_value.KeyValue{provider_kv})
	_, err = New(kv)
	suite.Require().Error(err)

	// network id should be string
	kv = key_value.Empty().
		Set("id", map[string]interface{}{}).
		Set("type", "evm").
		Set("providers", []key_value.KeyValue{provider_kv})
	_, err = New(kv)
	suite.Require().Error(err)

	// the provider type is not existing
	kv = key_value.Empty().
		Set("id", "1").
		Set("type", "not_existing").
		Set("providers", []key_value.KeyValue{provider_kv})
	_, err = New(kv)
	suite.Require().Error(err)

	// the provider type is not existing
	kv = key_value.Empty().
		Set("id", "1").
		Set("type", "not_existing").
		Set("providers", []key_value.KeyValue{provider_kv})
	_, err = New(kv)
	suite.Require().Error(err)

	// creating a network with type "ALL" is not supported
	kv = key_value.Empty().
		Set("id", "1").
		Set("type", "all").
		Set("providers", []key_value.KeyValue{provider_kv})
	_, err = New(kv)
	suite.Require().Error(err)

	// atleast one provider should be given
	kv = key_value.Empty().
		Set("id", "1").
		Set("type", "evm").
		Set("providers", []key_value.KeyValue{})
	_, err = New(kv)
	suite.Require().Error(err)

	// the provider type is invalid
	kv = key_value.Empty().
		Set("id", "1").
		Set("type", "evm").
		Set("providers", "not an array")
	_, err = New(kv)
	suite.Require().Error(err)

	// the providers parameter is not a list
	// as expected. but a kv
	kv = key_value.Empty().
		Set("id", "1").
		Set("type", "evm").
		Set("providers", provider_kv)
	_, err = New(kv)
	suite.Require().Error(err)

	// the correct Network
	network_id := "1"
	network_type := EVM

	provider_1 := key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", 3)
	provider_2 := key_value.Empty().
		Set("url", "https://example.com").
		Set("length", 32)
	kv = key_value.Empty().
		Set("id", network_id).
		Set("type", network_type.String()).
		Set("providers", []key_value.KeyValue{provider_1, provider_2})
	network, err := New(kv)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.network, *network)
}

func (suite *TestNetworkSuite) TestNetworks() {
	network_id := "1"
	network_type := EVM

	provider_1 := key_value.Empty().
		Set("url", "https://sample.com").
		Set("length", 3)
	provider_2 := key_value.Empty().
		Set("url", "https://example.com").
		Set("length", 32)
	kv := key_value.Empty().
		Set("id", network_id).
		Set("type", network_type.String()).
		Set("providers", []key_value.KeyValue{provider_1})

	// The equal network addition should fail
	_, err := NewNetworks([]key_value.KeyValue{kv, kv})
	suite.Require().Error(err)

	kv = key_value.Empty().
		Set("id", "1").
		Set("type", network_type.String()).
		Set("providers", []key_value.KeyValue{provider_1})
	kv_2 := key_value.Empty().
		Set("id", "2").
		Set("type", network_type.String()).
		Set("providers", []key_value.KeyValue{provider_2})

	networks, err := NewNetworks([]key_value.KeyValue{kv, kv_2})
	suite.Require().NoError(err)

	suite.Require().True(networks.Exist("1"))
	suite.Require().True(networks.Exist("2"))
	suite.Require().False(networks.Exist("3"))

	network_1, err := networks.Get("1")
	suite.Require().NoError(err)
	expected_1, _ := New(kv)
	suite.Require().EqualValues(expected_1, network_1)

	network_2, err := networks.Get("2")
	suite.Require().NoError(err)
	expected_2, _ := New(kv_2)
	suite.Require().EqualValues(expected_2, network_2)

	_, err = networks.Get("3")
	suite.Require().Error(err)
}

func (suite *TestNetworkSuite) TestStorage() {
	logger, err := log.New("test-suite", log.WITHOUT_TIMESTAMP)
	suite.Require().NoError(err)

	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)

	// no networks registered in the app configuration
	// therefore it should fail
	_, err = GetNetworks(app_config, ALL)
	suite.Require().Error(err)

	// invalid json
	networks_string := `
	[
		{"id": "56", "providers": [
			{
				"url": "https://rpc.ankr.com/bsc",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "1", "providers": [
			{
				"url": "https://eth.llamarpc.com",
				"length": 1000
			}
		], "type": "evm"},
	]`
	app_config.SetDefault(SDS_BLOCKCHAIN_NETWORKS, networks_string)
	_, err = GetNetworks(app_config, ALL)
	suite.Require().Error(err)

	// valid json, but missing some values
	networks_string = `
	[
		{"id": "56", "providers": [
			{
				"url": "https://rpc.ankr.com/bsc",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "1", "providers": [
			{
				"length": 1000
			}
		], "type": "evm"}
	]`
	app_config.SetDefault(SDS_BLOCKCHAIN_NETWORKS, networks_string)
	_, err = GetNetworks(app_config, ALL)
	suite.Require().Error(err)

	// valid json
	networks_string = `
	[
		{"id": "56", "providers": [
			{
				"url": "https://rpc.ankr.com/bsc",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "imx", "providers": [
			{
				"url": "https://eth.llamarpc.com",
				"length": 1000
			}
		], "type": "imx"}
	]`
	app_config.SetDefault(SDS_BLOCKCHAIN_NETWORKS, networks_string)
	networks, err := GetNetworks(app_config, ALL)
	suite.Require().NoError(err)
	suite.Require().Len(networks, 2)

	// the network type is invalid
	_, err = GetNetworks(app_config, "unsupported")
	suite.Require().Error(err)

	// there are only 1 EVM network
	networks, err = GetNetworks(app_config, EVM)
	suite.Require().NoError(err)
	suite.Require().Len(networks, 1)

	// there are only 1 IMX network
	networks, err = GetNetworks(app_config, IMX)
	suite.Require().NoError(err)
	suite.Require().Len(networks, 1)

	networks_string = `
	[
		{"id": "56", "providers": [
			{
				"url": "https://rpc.ankr.com/bsc",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "1", "providers": [
			{
				"url": "https://eth.llamarpc.com",
				"length": 1000
			}
		], "type": "evm"}
	]`
	app_config.SetDefault(SDS_BLOCKCHAIN_NETWORKS, networks_string)
	networks, err = GetNetworks(app_config, EVM)
	suite.Require().NoError(err)
	suite.Require().Len(networks, 2)

	// no imx networks in the list of networks
	networks, err = GetNetworks(app_config, IMX)
	suite.Require().NoError(err)
	suite.Require().Len(networks, 0)

	// network ids should be valid,
	// since its based on GetNetworks(*configuration.Config, NetworkType)
	network_ids, err := GetNetworkIds(app_config, ALL)
	suite.Require().NoError(err)
	suite.Require().Equal([]string{"56", "1"}, network_ids)

	network_ids, err = GetNetworkIds(app_config, EVM)
	suite.Require().NoError(err)
	suite.Require().Equal([]string{"56", "1"}, network_ids)

	network_ids, err = GetNetworkIds(app_config, IMX)
	suite.Require().NoError(err)
	suite.Require().Equal([]string{}, network_ids)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestNetwork(t *testing.T) {
	suite.Run(t, new(TestNetworkSuite))
}
