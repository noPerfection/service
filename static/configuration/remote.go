package configuration

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/static/smartcontract"
)

// get configuration from SDS Static by the configuration topic
func RemoteConfiguration(socket *remote.Socket, t *topic.Topic) (*Configuration, *smartcontract.Smartcontract, error) {
	kv, err := key_value.NewFromInterface(t)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert topic to key-value %v: %v", t, err)
	}

	request := message.Request{
		Command:    "configuration_get",
		Parameters: kv,
	}
	raw_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, nil, err
	}
	parameters := key_value.New(raw_parameters)

	raw_configuration, err := parameters.GetKeyValue("configuration")
	if err != nil {
		return nil, nil, err
	}
	raw_smartcontract, err := parameters.GetKeyValue("smartcontract")
	if err != nil {
		return nil, nil, err
	}
	conf, err := New(raw_configuration)
	if err != nil {
		return nil, nil, err
	}
	smartcontract, err := smartcontract.New(raw_smartcontract)
	if err != nil {
		return nil, nil, err
	}

	return conf, smartcontract, nil
}

// Send a command to the SDS Static to register a new configuration
func RemoteConfigurationRegister(socket *remote.Socket, conf *Configuration) error {
	kv, err := key_value.NewFromInterface(conf)
	if err != nil {
		return fmt.Errorf("failed to convert static.Configuration to KeyValue: %v", err)
	}

	request := message.Request{
		Command:    "configuration_register",
		Parameters: kv,
	}

	_, err = socket.RequestRemoteService(&request)
	return err
}
