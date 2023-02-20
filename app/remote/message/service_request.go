package message

import (
	"fmt"

	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// The SDS Service will accepts a request from another request
type ServiceRequest struct {
	PublicKey  string             `json:"public_key"`
	Command    string             `json:"command"`    // Command type
	Parameters key_value.KeyValue `json:"parameters"` // Parameters of the request
	service    *service.Service   // The service parameters
}

// ServiceRequest message as a  sequence of bytes
func (request *ServiceRequest) ToBytes() ([]byte, error) {
	kv, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize ServiceRequest to key-value %v: %v", request, err)
	}

	return kv.ToBytes()
}

func (request *ServiceRequest) Service() *service.Service {
	return request.service
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
		return ServiceRequest{}, fmt.Errorf("failed to convert message %s to key-value %v", msg, err)
	}

	var request ServiceRequest
	err = data.ToInterface(&request)
	if err != nil {
		return ServiceRequest{}, fmt.Errorf("failed to convert key-value %v for message %s to intermediate interface: %v", data, msg, err)
	}

	// The developers or smartcontract developer public keys are not in the environment variable
	// as a servie.
	service_env, err := service.GetByPublicKey(request.PublicKey)
	if err != nil {
		return ServiceRequest{}, fmt.Errorf("service associated with public key %s not found: %v", request.PublicKey, err)
	}

	request.service = service_env

	return request, nil
}
