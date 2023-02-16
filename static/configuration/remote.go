package configuration

import (
	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
	"github.com/blocklords/gosds/static/smartcontract"
	"github.com/blocklords/gosds/topic"
)

// get configuration from SDS Static by the configuration topic
func RemoteConfiguration(socket *remote.Socket, t *topic.Topic) (*Configuration, *smartcontract.Smartcontract, error) {
	// Send hello.
	request := message.Request{
		Command:    "configuration_get",
		Parameters: t.ToJSON(),
	}
	parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, nil, err
	}

	raw_configuration, err := message.GetMap(parameters, "configuration")
	if err != nil {
		return nil, nil, err
	}
	raw_smartcontract, err := message.GetMap(parameters, "smartcontract")
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
	// Send hello.
	request := message.Request{
		Command:    "configuration_register",
		Parameters: conf.ToJSON(),
	}

	_, err := socket.RequestRemoteService(&request)
	return err
}
