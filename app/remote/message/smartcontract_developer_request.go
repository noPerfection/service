package message

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
)

// The SDS Service will accepts the SmartcontractDeveloperRequest message.
// Its created from message.Request.
// Therefore we don't serialize or deserialize it.
type SmartcontractDeveloperRequest struct {
	Address        string
	NonceTimestamp uint64
	Signature      string
	Request        Request
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
func (sm_req *SmartcontractDeveloperRequest) message_hash() ([]byte, error) {
	req := Request{
		Command:    sm_req.Request.Command,
		Parameters: sm_req.Request.Parameters,
	}
	req.Parameters = req.Parameters.Set("_nonce_timestamp", sm_req.NonceTimestamp)
	req.Parameters = req.Parameters.Set("_address", sm_req.Address)

	json_object, err := key_value.NewFromInterface(req)
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

// Extracts the smartcontract request parameters from the request
// The request then cleaned up from the smartcontract request parameters
func ToSmartcontractDeveloperRequest(request Request) (SmartcontractDeveloperRequest, error) {
	address, err := request.Parameters.GetString("_address")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetString(`address`): %w", err)
	} else {
		delete(request.Parameters, "_address")
	}

	nonce_timestamp, err := request.Parameters.GetUint64("_nonce_timestamp")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetUint64(`_nonce_timestamp`): %w", err)
	} else {
		delete(request.Parameters, "_nonce_timestamp")
	}

	signature, err := request.Parameters.GetString("_signature")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetString(`_signature`): %w", err)
	} else {
		delete(request.Parameters, "_signature")
	}

	sm_request := SmartcontractDeveloperRequest{
		Address:        address,
		NonceTimestamp: nonce_timestamp,
		Signature:      signature,
		Request:        request,
	}

	return sm_request, nil
}
