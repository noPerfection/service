package configuration

import "github.com/blocklords/gosds/message"

// Creates a new static.Configuration class based on the given data
func New(parameters map[string]interface{}) (*Configuration, error) {
	organization, err := message.GetString(parameters, "o")
	if err != nil {
		return nil, err
	}
	project, err := message.GetString(parameters, "p")
	if err != nil {
		return nil, err
	}
	network_id, err := message.GetString(parameters, "n")
	if err != nil {
		return nil, err
	}
	group, err := message.GetString(parameters, "g")
	if err != nil {
		return nil, err
	}
	smartcontract_name, err := message.GetString(parameters, "s")
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
	address, err := message.GetString(parameters, "address")
	if err == nil {
		conf.SetAddress(address)
	}
	id, err := message.GetUint64(parameters, "id")
	if err == nil {
		conf.SetId(uint(id))
	}

	return &conf, nil
}
