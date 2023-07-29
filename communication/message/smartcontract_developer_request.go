package message

import (
	"fmt"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
)

// SmartcontractDeveloperRequest is the message sent by smartcontract developer.
// It's created from message.Request.
// Therefore, we don't serialize or deserialize it.
//
// Unlike other message types, this Request
// doesn't have the parse.
// Instead, it's derived from message.ToSmartcontractDeveloper()
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
// But it's done by the app/account.SmartcontractDeveloper
// Because of the encryption algorithm depends on the blockchain, to validate the nonce.
type SmartcontractDeveloperRequest struct {
	Address        string
	NonceTimestamp uint64
	Signature      string
	Request        Request
}

// Bytes message as a sequence of bytes
//
// For now .Bytes() is not used by the service.
func (smReq *SmartcontractDeveloperRequest) Bytes() ([]byte, error) {
	kv, err := key_value.NewFromInterface(smReq)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize SmartcontractDeveloper Request to key-value %v: %v", smReq, err)
	}

	bytes, err := kv.Bytes()
	if err != nil {
		return nil, fmt.Errorf("kv.Bytes: %w", err)
	}

	return bytes, nil
}

// String Converts SmartcontractDeveloperRequest message to the string
//
// For now .String() is not used by the service.
func (smReq *SmartcontractDeveloperRequest) String() (string, error) {
	bytes, err := smReq.Bytes()
	if err != nil {
		return "", fmt.Errorf("request.Bytes: %w", err)
	}

	return string(bytes), nil
}

// Gets the message without a prefix.
// The message is a JSON representation of the Request but without "signature" parameter.
// Converted into the hash using Keccak32.
//
// The request parameters are ordered in an alphanumerical order.
func (smReq *SmartcontractDeveloperRequest) messageHash() ([]byte, error) {
	req := Request{
		Command:    smReq.Request.Command,
		Parameters: smReq.Request.Parameters,
	}
	req.Parameters = req.Parameters.Set("_nonce_timestamp", smReq.NonceTimestamp)
	req.Parameters = req.Parameters.Set("_address", smReq.Address)

	jsonObject, err := key_value.NewFromInterface(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize %v", err)
	}
	delete(jsonObject, "signature")

	bytes, err := key_value.New(jsonObject).Bytes()
	if err != nil {
		return []byte{}, fmt.Errorf("key_value.Bytes: %w", err)
	}

	hash := crypto.Keccak256Hash(bytes)

	return hash.Bytes(), nil
}

// DigestedMessage Gets the digested message with a prefix that is ready to sign or verify.
// For ethereum blockchain accounts it also adds the prefix "\x19Ethereum Signed Message:\n"
func (smReq *SmartcontractDeveloperRequest) DigestedMessage() ([]byte, error) {
	messageHash, err := smReq.messageHash()
	if err != nil {
		return []byte{}, fmt.Errorf("request.message_hash: %w", err)
	}
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digestedHash := crypto.Keccak256Hash(append(prefix, messageHash...))
	return digestedHash.Bytes(), nil
}

func (smReq *SmartcontractDeveloperRequest) validateParameters() error {
	if len(smReq.Address) < 3 {
		return fmt.Errorf("atleast 3 characters required for address")
	} else if smReq.Address[:2] != "0x" && smReq.Address[:2] != "0X" {
		return fmt.Errorf("'%s' address parameter has no '0x' prefix", smReq.Address)
	}
	if len(smReq.Signature) < 3 {
		return fmt.Errorf("atleast 3 characters required for signature")
	} else if smReq.Signature[:2] != "0x" && smReq.Signature[:2] != "0X" {
		return fmt.Errorf("'%s' signature parameter has no '0x' prefix", smReq.Signature)
	}
	if smReq.NonceTimestamp == 0 {
		return fmt.Errorf("nonce can not be 0")
	}

	return nil
}

// ToSmartcontractDeveloperRequest Extracts the smartcontract request parameters from the request
//
//	then cleaned up from the smartcontract request parameters
func ToSmartcontractDeveloperRequest(request Request) (SmartcontractDeveloperRequest, error) {
	_, err := request.Bytes()
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("request is invalid: %w", err)
	}

	address, err := request.Parameters.GetString("_address")
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("GetString(`address`): %w", err)
	} else {
		delete(request.Parameters, "_address")
	}

	nonceTimestamp, err := request.Parameters.GetUint64("_nonce_timestamp")
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

	smRequest := SmartcontractDeveloperRequest{
		Address:        address,
		NonceTimestamp: nonceTimestamp,
		Signature:      signature,
		Request:        request,
	}

	err = smRequest.validateParameters()
	if err != nil {
		return SmartcontractDeveloperRequest{}, fmt.Errorf("validation: %w", err)
	}

	return smRequest, nil
}
