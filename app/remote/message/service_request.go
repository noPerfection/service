package message

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// The SDS Service will accepts a request from another request
type ServiceRequest struct {
	Service    *service.Service       // The service parameters
	Command    string                 // Command type
	Parameters map[string]interface{} // Parameters of the request
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
func (request *ServiceRequest) ToBytes() []byte {
	interfaces := request.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		fmt.Println("error while converting json into bytes", err)
		return []byte{}
	}

	return byt
}

// Convert ServiceRequest message to the string
func (request *ServiceRequest) ToString() string {
	return string(request.ToBytes())
}

// Parse the messages from zeromq into the ServiceRequest
func ParseServiceRequest(msgs []string) (ServiceRequest, error) {
	msg := ToString(msgs)

	var dat key_value.KeyValue

	decoder := json.NewDecoder(strings.NewReader(msg))
	decoder.UseNumber()

	if err := decoder.Decode(&dat); err != nil {
		return ServiceRequest{}, err
	}

	command, err := dat.GetString("command")
	if err != nil {
		return ServiceRequest{}, err
	}
	parameters, err := dat.GetMap("parameters")
	if err != nil {
		return ServiceRequest{}, err
	}

	public_key, err := dat.GetString("public_key")
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
