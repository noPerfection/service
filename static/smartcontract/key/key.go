// Smartcontract key is the unique identifier within the SeascapeSDS
// Its composed as network_id + "." + address
package key

import (
	"fmt"
	"strings"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// network id + "." + address
type Key string

// map(smartcontract_key => topic_string)
type KeyToTopicString map[Key]string

// Creates a new smartcontract key
func New(network_id string, address string) Key {
	return Key(network_id + "." + address)
}

// Creates a new smartcontract key from the map
func NewFromKeyValue(kv key_value.KeyValue) (Key, error) {
	network_id, err := kv.GetString("network_id")
	if err != nil {
		return "", fmt.Errorf("missing 'network_id' parameter")
	}
	address, err := kv.GetString("address")
	if err != nil {
		return "", fmt.Errorf("missing 'network_id' parameter")
	}

	return New(network_id, address), nil
}

// converts the string to Key
func NewFromString(s string) Key {
	return Key(s)
}

// The smartcontract parameters that composes the smartcontract key
// its the network id and the address are returns
func (k *Key) Decompose() (string, string) {
	str := strings.Split(string(*k), ".")
	return str[0], str[1]
}

func (k *Key) NetworkId() string {
	str := strings.Split(string(*k), ".")
	return str[0]
}

func (k *Key) Address() string {
	str := strings.Split(string(*k), ".")
	return str[1]
}

// Returns the key as a string
func (k *Key) ToString() string {
	return string(*k)
}
