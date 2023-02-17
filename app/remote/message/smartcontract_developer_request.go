package message

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
)

// The SDS Service will accepts the SmartcontractDeveloperRequest message.
type SmartcontractDeveloperRequest struct {
	Address        string                 // The whitelisted address of the user
	NonceTimestamp uint64                 // Nonce as a unix timestamp in seconds
	Signature      string                 // Command, nonce, address and parameters signed together
	Command        string                 // Command type
	Parameters     map[string]interface{} // Parameters of the request
}

// Convert SmartcontractDeveloperRequest to JSON
func (request *SmartcontractDeveloperRequest) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"address":         request.Address,
		"nonce_timestamp": request.NonceTimestamp,
		"signature":       request.Signature,
		"command":         request.Command,
		"parameters":      request.Parameters,
	}
}

// SmartcontractDeveloperRequest message as a  sequence of bytes
func (request *SmartcontractDeveloperRequest) ToBytes() []byte {
	interfaces := request.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		fmt.Println("error while converting json into bytes", err)
		return []byte{}
	}

	return byt
}

// Convert SmartcontractDeveloperRequest message to the string
func (request *SmartcontractDeveloperRequest) ToString() string {
	return string(request.ToBytes())
}

// Gets the message without a prefix.
// The message is a JSON represantion of the Request but without "signature" parameter.
// Converted into the hash using Keccak32.
//
// The request parameters are oredered in an alphanumerical order.
func (request *SmartcontractDeveloperRequest) message_hash() []byte {
	json_object := request.ToJSON()
	delete(json_object, "signature")
	bytes, err := json.Marshal(json_object)
	if err != nil {
		fmt.Println("error while converting json into bytes", err)
		return []byte{}
	}

	hash := crypto.Keccak256Hash(bytes)

	return hash.Bytes()
}

// Gets the digested message with a prefix
// For ethereum the prefix is "\x19Ethereum Signed Message:\n"
func (request *SmartcontractDeveloperRequest) DigestedMessage() []byte {
	message_hash := request.message_hash()
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digested_hash := crypto.Keccak256Hash(append(prefix, message_hash...))
	return digested_hash.Bytes()
}

// Parse the messages from zeromq into the SmartcontractDeveloperRequest
func ParseSmartcontractDeveloperRequest(msgs []string) (SmartcontractDeveloperRequest, error) {
	msg := ToString(msgs)

	var dat key_value.KeyValue

	decoder := json.NewDecoder(strings.NewReader(msg))
	decoder.UseNumber()

	if err := decoder.Decode(&dat); err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	command, err := dat.GetString("command")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}
	parameters, err := dat.GetMap("parameters")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	address, err := dat.GetString("address")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	nonce_timestamp, err := dat.GetUint64("nonce_timestamp")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	signature, err := dat.GetString("signature")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	request := SmartcontractDeveloperRequest{
		Address:        address,
		NonceTimestamp: nonce_timestamp,
		Signature:      signature,
		Command:        command,
		Parameters:     parameters,
	}

	return request, nil
}
