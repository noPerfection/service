package configuration

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Creates a new static.Configuration class based on the given data
func New(parameters key_value.KeyValue) (*Configuration, error) {
	organization, err := parameters.GetString("o")
	if err != nil {
		return nil, err
	}
	project, err := parameters.GetString("p")
	if err != nil {
		return nil, err
	}
	network_id, err := parameters.GetString("n")
	if err != nil {
		return nil, err
	}
	group, err := parameters.GetString("g")
	if err != nil {
		return nil, err
	}
	smartcontract_name, err := parameters.GetString("s")
	if err != nil {
		return nil, err
	}

	conf := Configuration{
		Organization: organization,
		Project:      project,
		NetworkId:    network_id,
		Group:        group,
		Name:         smartcontract_name,
		exists:       true,
	}
	address, err := parameters.GetString("address")
	if err == nil {
		conf.SetAddress(address)
	}
	id, err := parameters.GetUint64("id")
	if err == nil {
		conf.SetId(uint(id))
	}

	return &conf, nil
}
