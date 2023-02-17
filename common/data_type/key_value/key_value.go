package key_value

import (
	"encoding/json"
	"errors"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common/math"
)

// identical to the golang map
type KeyValue map[string]interface{}

// Converts the map to the key-value data type
func NewKeyValue(key_value map[string]interface{}) KeyValue {
	return KeyValue(key_value)
}

// Converts the key-valueto the golang map
func (k KeyValue) ToMap() map[string]interface{} {
	return map[string]interface{}(k)
}

// Returns the all uint64 parameters
func (parameters KeyValue) GetUint64s(names ...string) ([]uint64, error) {
	numbers := make([]uint64, len(names))
	for i, name := range names {
		number, err := parameters.GetUint64(name)
		if err != nil {
			return nil, err
		}

		numbers[i] = number
	}

	return numbers, nil
}

// Returns the all float64 parameters
func (parameters KeyValue) GetFloat64s(names ...string) ([]float64, error) {
	numbers := make([]float64, len(names))
	for i, name := range names {
		number, err := parameters.GetFloat64(name)
		if err != nil {
			return nil, err
		}

		numbers[i] = number
	}

	return numbers, nil
}

// Returns the all string parameters
func (parameters KeyValue) GetStrings(names ...string) ([]string, error) {
	values := make([]string, len(names))
	for i, name := range names {
		value, err := parameters.GetString(name)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return values, nil
}

// Returns the all big numbers
func (parameters KeyValue) GetBigNumbers(names ...string) ([]*big.Int, error) {
	values := make([]*big.Int, len(names))
	for i, name := range names {
		value, err := parameters.GetBigNumber(name)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return values, nil
}

// Returns the all string lists
func (parameters KeyValue) GetStringLists(names ...string) ([][]string, error) {
	values := make([][]string, len(names))
	for i, name := range names {
		value, err := parameters.GetStringList(name)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return values, nil
}

// Returns the all map lists
func (parameters KeyValue) GetMapLists(names ...string) ([][]map[string]interface{}, error) {
	values := make([][]map[string]interface{}, len(names))
	for i, name := range names {
		value, err := parameters.GetMapList(name)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return values, nil
}

// Returns the all maps
func (parameters KeyValue) GetMaps(names ...string) ([]map[string]interface{}, error) {
	values := make([]map[string]interface{}, len(names))
	for i, name := range names {
		value, err := parameters.GetMap(name)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return values, nil
}

// Returns the parameter as an uint64
func (parameters KeyValue) GetUint64(name string) (uint64, error) {
	raw, exists := parameters[name]
	if !exists {
		return 0, errors.New("missing '" + name + "' parameter in the Request")
	}

	pure_value, ok := raw.(uint64)
	if ok {
		return pure_value, nil
	}
	value, ok := raw.(json.Number)
	if !ok {
		return 0, errors.New("parameter '" + name + "' expected to be as a number")
	}

	number, err := strconv.ParseUint(string(value), 10, 64)

	return number, err
}

func (parameters KeyValue) GetFloat64(name string) (float64, error) {
	raw, exists := parameters[name]
	if !exists {
		return 0, errors.New("missing '" + name + "' parameter in the Request")
	}
	pure_value, ok := raw.(float64)
	if ok {
		return pure_value, nil
	}
	value, err := raw.(json.Number).Float64()

	return value, err
}

func (parameters KeyValue) GetBoolean(name string) (bool, error) {
	raw, exists := parameters[name]
	if !exists {
		return false, errors.New("missing '" + name + "' parameter in the Request")
	}

	pure_value, ok := raw.(bool)
	if ok {
		return pure_value, nil
	}

	return false, errors.New("the '" + name + "' is not in a boolean format")
}

// Returns the parsed large number. If the number size is more than 64 bits.
func (parameters KeyValue) GetBigNumber(name string) (*big.Int, error) {
	raw, exists := parameters[name]
	if !exists {
		return nil, errors.New("missing '" + name + "' parameter in the Request")
	}

	value, ok := raw.(json.Number)
	if !ok {
		return nil, errors.New("parameter '" + name + "' expected to be as a number")
	}

	number, ok := math.ParseBig256(string(value))
	if !ok {
		return nil, errors.New("parameter '" + name + "' is not a big number")
	}

	return number, nil
}

// Returns the paramater as a string
func (parameters KeyValue) GetString(name string) (string, error) {
	raw, exists := parameters[name]
	if !exists {
		return "", errors.New("missing '" + name + "' parameter in the Request")
	}
	value, ok := raw.(string)
	if !ok {
		return "", errors.New("expected string type for '" + name + "' parameter")
	}

	return value, nil
}

// Returns list of strings
func (parameters KeyValue) GetStringList(name string) ([]string, error) {
	raw, exists := parameters[name]
	if !exists {
		return nil, errors.New("missing '" + name + "' parameter in the Request")
	}

	values, ok := raw.([]interface{})
	if !ok {
		ready_list, ok := raw.([]string)
		if !ok {
			return nil, errors.New("expected list type for '" + name + "' parameter")
		} else {
			return ready_list, nil
		}
	}

	list := make([]string, len(values))
	for i, raw_value := range values {
		v, ok := raw_value.(string)
		if !ok {
			return nil, errors.New("one of the elements in the parameter is not a string")
		}

		list[i] = v
	}

	return list, nil
}

// Returns the parameter as a slice of map:
//
// []map[string]interface{}
func (parameters KeyValue) GetMapList(name string) ([]map[string]interface{}, error) {
	raw, exists := parameters[name]
	if !exists {
		return nil, errors.New("missing '" + name + "' parameter in the Request")
	}
	values, ok := raw.([]interface{})
	if !ok {
		ready_list, ok := raw.([]map[string]interface{})
		if !ok {
			return nil, errors.New("expected list type for '" + name + "' parameter")
		} else {
			return ready_list, nil
		}
	}

	list := make([]map[string]interface{}, len(values))
	for i, raw_value := range values {
		v, ok := raw_value.(map[string]interface{})
		if !ok {
			return nil, errors.New("one of the elements in the parameter is not a map")
		}

		list[i] = v
	}

	return list, nil
}

// Returns the parameter as a map:
//
// map[string]interface{}
func (parameters KeyValue) GetMap(name string) (map[string]interface{}, error) {
	raw, exists := parameters[name]
	if !exists {
		return nil, errors.New("missing '" + name + "' parameter in the Request")
	}
	value, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("expected map type for '" + name + "' parameter")
	}

	return value, nil
}
