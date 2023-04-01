// Smartcontract key is the unique identifier within the SeascapeSDS
// Its composed as network_id + "." + address
package smartcontract_key

import (
	"fmt"
	"strings"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// network id + "." + address
type Key struct {
	NetworkId string `json:"network_id"`
	Address   string `json:"address"`
}

// map(smartcontract_key => topic_string)
type KeyToTopicString map[Key]string

// Creates a new smartcontract key
func New(network_id string, address string) Key {
	return Key{NetworkId: network_id, Address: address}
}

// Creates a new smartcontract key from the map
func NewFromKeyValue(parameters key_value.KeyValue) (Key, error) {
	var key Key
	err := parameters.ToInterface(&key)
	if err != nil {
		return Key{}, fmt.Errorf("failed to convert key-value to interface %v", err)
	}

	if len(key.NetworkId) == 0 ||
		len(key.Address) == 0 {
		return Key{}, fmt.Errorf("missing parameter or empty parameter")
	}

	return key, nil
}

// converts the string to Key
func NewFromString(s string) (Key, error) {
	str := strings.Split(s, ".")
	if len(str) != 2 {
		return Key{}, fmt.Errorf("string '%s' doesn't have two parts", s)
	}

	if len(str[0]) == 0 ||
		len(str[1]) == 0 {
		return Key{}, fmt.Errorf("missing parameter or empty parameter")
	}

	return Key{NetworkId: str[0], Address: str[1]}, nil
}

// Returns the key as a string
// `<network_id>.<address>`
func (k *Key) ToString() string {
	return k.NetworkId + "." + k.Address
}
