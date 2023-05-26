package message

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
)

// The SDS Service will accepts the SmartcontractDeveloperRequest message.
// Its created from message.Request.
// Therefore we don't serialize or deserialize it.
//
// Unlike other message types, this Request
// doesn't have the parse.
// Instead its derived from message.ToSmartcontractDeveloper()
//
// The correct request message should have the following
// parameters:
//   - _address in a hex format with "0x" prefixed. Derived from public key
//   - _nonce_timestamp a number
//   - _signature in a hex format. Signature of the digested
//     hash signed by the private key.
//     The digested hash is the hash of message hash + prefix.
//     The message hash is the request parameter except the _signature parameter
//
// The Valid Request to convert:
//
//		request := Request{
//			Command: "get_data",
//			Parameters: key_value.Empty().
//				Set("_address", "0xdead").
//				Set("_nonce_timestamp", 14790213).
//				Set("_signature", "0xdead").
//				// the "get_data" command parameters...
//				Set("data_type", "number")
//		}
//
//	 sm_request, _ := message.ToSmartcontractDeveloperRequest(request)
//
// ---------------------------------------------
//
// The signature and address verification is not checked here
// But its done by the app/account.SmartcontractDeveloper
// Because of the encryption algorithm depends on the blockchain, to validate the nonce.
type SmartcontractDeveloperRequest struct {
	Address        string
	NonceTimestamp uint64
	Signature      string
	Request        Request
}

// SmartcontractDeveloperRequest message as a sequence of bytes
//
// For now .ToBytes() is not used by the service.
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
//
// For now .ToString() is not used by the service.
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

// Gets the digested message with a prefix that is ready to sign or verify.
// For ethereum blockchain accounts it also adds the prefix "\x19Ethereum Signed Message:\n"
func (request *SmartcontractDeveloperRequest) DigestedMessage() ([]byte, error) {
	message_hash, err := request.message_hash()
	if err != nil {
		return []byte{}, fmt.Errorf("request.message_hash: %w", err)
	}
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digested_hash := crypto.Keccak256Hash(append(prefix, message_hash...))
	return digested_hash.Bytes(), nil
}

func (sm_req *SmartcontractDeveloperRequest) validate_parameters() error {
	if len(sm_req.Address) < 3 {
		return fmt.Errorf("atleast 3 characters required for address")
	} else if sm_req.Address[:2] != "0x" && sm_req.Address[:2] != "0X" {
		return fmt.Errorf("'%s' address parameter has no '0x' prefix", sm_req.Address)
	}
	if len(sm_req.Signature) < 3 {
		return fmt.Errorf("atleast 3 characters required for signature")
	} else if sm_req.Signature[:2] != "0x" && sm_req.Signature[:2] != "0X" {
		return fmt.Errorf("'%s' signature parameter has no '0x' prefix", sm_req.Signature)
	}
	if sm_req.NonceTimestamp == 0 {
		return fmt.Errorf("nonce can not be 0")
	}

	return nil
}

// Extracts the smartcontract request parameters from the request
// The request then cleaned up from the smartcontract request parameters
func ToSmartcontractDeveloperRequest(request Request) (SmartcontractDeveloperRequest, error) {
	_, err := request.ToBytes()
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("request is invalid: %w", err)
	}

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

	err = sm_request.validate_parameters()
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("validation: %w", err)
	}

	return sm_request, nil
}
