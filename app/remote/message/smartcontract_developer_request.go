package message

import (
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
)

// The SDS Service will accepts the SmartcontractDeveloperRequest message.
type SmartcontractDeveloperRequest struct {
	Address        string             `json:"address"`         // The whitelisted address of the user
	NonceTimestamp uint64             `json:"nonce_timestamp"` // The timestamp           // Nonce as a unix timestamp in seconds
	Signature      string             `json:"signature"`       // The signature           // Command, nonce, address and parameters signed together
	Command        string             `json:"command"`         // The command           // Command type
	Parameters     key_value.KeyValue `json:"parameters"`      // The parametersParameters of the request
}

// SmartcontractDeveloperRequest message as a  sequence of bytes
func (request *SmartcontractDeveloperRequest) ToBytes() ([]byte, error) {
	kv, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize SmartcontractDeveloper Request to key-value %v: %v", request, err)
	}

	bytes, err := kv.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("kv.ToBytes: %w", err)
	}

	return bytes, nil
}

// Convert SmartcontractDeveloperRequest message to the string
func (request *SmartcontractDeveloperRequest) ToString() (string, error) {
	bytes, err := request.ToBytes()
	if err != nil {
		return "", fmt.Errorf("request.ToBytes: %w", err)
	}

	return string(bytes), nil
}

// Gets the message without a prefix.
// The message is a JSON represantion of the Request but without "signature" parameter.
// Converted into the hash using Keccak32.
//
// The request parameters are oredered in an alphanumerical order.
func (request *SmartcontractDeveloperRequest) message_hash() ([]byte, error) {
	json_object, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize %v", err)
	}
	delete(json_object, "signature")

	bytes, err := key_value.New(json_object).ToBytes()
	if err != nil {
		return []byte{}, fmt.Errorf("key_value.ToBytes: %w", err)
	}

	hash := crypto.Keccak256Hash(bytes)

	return hash.Bytes(), nil
}

// Gets the digested message with a prefix
// For ethereum the prefix is "\x19Ethereum Signed Message:\n"
func (request *SmartcontractDeveloperRequest) DigestedMessage() ([]byte, error) {
	message_hash, err := request.message_hash()
	if err != nil {
		return []byte{}, fmt.Errorf("request.message_hash: %w", err)
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
		return SmartcontractDeveloperRequest{}, fmt.Errorf("key_value.NewFromString: %w", err)
	}

	command, err := data.GetString("command")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetString(`command`): %w", err)
	}
	parameters, err := data.GetKeyValue("parameters")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetKeyValue(`parameters`): %w", err)
	}

	address, err := data.GetString("address")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetString(`address`): %w", err)
	}

	nonce_timestamp, err := data.GetUint64("nonce_timestamp")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetUint64(`nonce_timestamp`): %w", err)
	}

	signature, err := data.GetString("signature")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetString(`signature`): %w", err)
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
