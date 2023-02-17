package message

import (
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// The SDS Service will accepts a request from another request
type ServiceRequest struct {
	Service    *service.Service   // The service parameters
	Command    string             // Command type
	Parameters key_value.KeyValue // Parameters of the request
}

func (request *ServiceRequest) CommandName() string {
	return request.Command
}

// Convert ServiceRequest to JSON
func (request *ServiceRequest) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"public_key": request.Service.PublicKey,
		"command":    request.Command,
		"parameters": request.Parameters,
	}
}

// ServiceRequest message as a  sequence of bytes
func (request *ServiceRequest) ToBytes() ([]byte, error) {
	return key_value.New(request.ToJSON()).ToBytes()
}

// Convert ServiceRequest message to the string
func (request *ServiceRequest) ToString() (string, error) {
	bytes, err := request.ToBytes()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Parse the messages from zeromq into the ServiceRequest
func ParseServiceRequest(msgs []string) (ServiceRequest, error) {
	msg := ToString(msgs)

	data, err := key_value.NewFromString(msg)
	if err != nil {
		return ServiceRequest{}, err
	}

	command, err := data.GetString("command")
	if err != nil {
		return ServiceRequest{}, err
	}
	parameters, err := data.GetKeyValue("parameters")
	if err != nil {
		return ServiceRequest{}, err
	}

	public_key, err := data.GetString("public_key")
	if err != nil {
		return ServiceRequest{}, err
	}

	// The developers or smartcontract developer public keys are not in the environment variable
	// as a servie.
	service_env, err := service.GetByPublicKey(public_key)
	if err != nil {
		return ServiceRequest{}, err
	}

	request := ServiceRequest{
		Service:    service_env,
		Command:    command,
		Parameters: parameters,
	}

	return request, nil
}
