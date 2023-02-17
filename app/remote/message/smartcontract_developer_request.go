package message

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
)

// The SDS Service will accepts the SmartcontractDeveloperRequest message.
type SmartcontractDeveloperRequest struct {
	Address        string             // The whitelisted address of the user
	NonceTimestamp uint64             // Nonce as a unix timestamp in seconds
	Signature      string             // Command, nonce, address and parameters signed together
	Command        string             // Command type
	Parameters     key_value.KeyValue // Parameters of the request
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
func (request *SmartcontractDeveloperRequest) ToBytes() ([]byte, error) {
	return key_value.New(request.ToJSON()).ToBytes()
}

// Convert SmartcontractDeveloperRequest message to the string
func (request *SmartcontractDeveloperRequest) ToString() (string, error) {
	bytes, err := request.ToBytes()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Gets the message without a prefix.
// The message is a JSON represantion of the Request but without "signature" parameter.
// Converted into the hash using Keccak32.
//
// The request parameters are oredered in an alphanumerical order.
func (request *SmartcontractDeveloperRequest) message_hash() ([]byte, error) {
	json_object := request.ToJSON()
	delete(json_object, "signature")

	bytes, err := key_value.New(json_object).ToBytes()
	if err != nil {
		return []byte{}, err
	}

	hash := crypto.Keccak256Hash(bytes)

	return hash.Bytes(), nil
}

// Gets the digested message with a prefix
// For ethereum the prefix is "\x19Ethereum Signed Message:\n"
func (request *SmartcontractDeveloperRequest) DigestedMessage() ([]byte, error) {
	message_hash, err := request.message_hash()
	if err != nil {
		return []byte{}, err
	}
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digested_hash := crypto.Keccak256Hash(append(prefix, message_hash...))
	return digested_hash.Bytes(), nil
}

// Parse the messages from zeromq into the SmartcontractDeveloperRequest
func ParseSmartcontractDeveloperRequest(msgs []string) (SmartcontractDeveloperRequest, error) {
	msg := ToString(msgs)

	data, err := key_value.NewFromString(msg)
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	command, err := data.GetString("command")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}
	parameters, err := data.GetKeyValue("parameters")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	address, err := data.GetString("address")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	nonce_timestamp, err := data.GetUint64("nonce_timestamp")
	if err != nil {
		return SmartcontractDeveloperRequest{}, err
	}

	signature, err := data.GetString("signature")
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
