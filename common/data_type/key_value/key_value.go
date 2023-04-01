package key_value

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/math"
)

// identical to the golang map but with the helper
// functions.
// no value could be `nil`.
type KeyValue map[string]interface{}

// Converts the map to the key-value data type
func New(key_value map[string]interface{}) KeyValue {
	return KeyValue(key_value)
}

// Converts the s string with a json decoder into the key value
func NewFromString(s string) (KeyValue, error) {
	var key_value KeyValue

	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()

	if err := decoder.Decode(&key_value); err != nil {
		return Empty(), fmt.Errorf("json.decoder: '%w'", err)
	}

	nil_err := key_value.no_nil_value()
	if nil_err != nil {
		return Empty(), fmt.Errorf("value is nil: %w", nil_err)
	}

	return key_value, nil
}

// Converts the data structure "i" to KeyValue
// In order to do that, it serializes data structure using json
//
// The data structures should define the json variable names
func NewFromInterface(i interface{}) (KeyValue, error) {
	var k KeyValue
	bytes, err := json.Marshal(i)
	if err != nil {
		return Empty(), fmt.Errorf("json.marshal %T: '%w'", i, err)
	}
	err = json.Unmarshal(bytes, &k)
	if err != nil {
		return Empty(), fmt.Errorf("json:unmarshal %s: '%w'", bytes, err)
	}

	nil_err := k.no_nil_value()
	if nil_err != nil {
		return Empty(), fmt.Errorf("value is nil: %w", nil_err)
	}

	return k, nil
}

// Returns an empty key value
func Empty() KeyValue {
	return KeyValue(map[string]interface{}{})
}

// Checks that the values are not nil.
func (k KeyValue) no_nil_value() error {
	for key, value := range k {
		if value == nil {
			return fmt.Errorf("key %s is nil", key)
		}

		nested_kv, ok := value.(KeyValue)

		if ok {
			err := nested_kv.no_nil_value()
			if err != nil {
				return fmt.Errorf("key %s nested value nil: %w", key, err)
			}

			continue
		}

		nested_map, ok := value.(map[string]interface{})

		if ok {
			nested_kv = New(nested_map)

			err := nested_kv.no_nil_value()
			if err != nil {
				return fmt.Errorf("key %s nested value nil: %w", key, err)
			}
		}
	}

	return nil
}

// It sets the numbers in a string format.
// The string format for the number means a json number
func (k KeyValue) set_number() {
	for key, value := range k {
		if value == nil {
			continue
		}

		// even if its a number wrapped as a string
		// we won't convert it.
		_, ok := value.(string)
		if ok {
			continue
		}

		big_num, err := k.GetBigNumber(key)
		if err == nil {
			delete(k, key)

			json_number := json.Number(big_num.String())
			k.Set(key, json_number)
			continue
		}

		float_num, err := k.GetFloat64(key)
		if err == nil {
			delete(k, key)

			json_number := json.Number(strconv.FormatFloat(float_num, 'G', -1, 64))
			k.Set(key, json_number)
			continue
		}

		num, err := k.GetUint64(key)
		if err == nil {
			delete(k, key)

			json_number := json.Number(strconv.FormatUint(num, 10))
			k.Set(key, json_number)
			continue
		}

		nested_kv, ok := value.(KeyValue)
		if ok {
			nested_kv.set_number()

			delete(k, key)
			k.Set(key, nested_kv)
			continue
		}

		nested_map, ok := value.(map[string]interface{})
		if ok {
			nested_kv = New(nested_map)
			// ToMap will call set_number()
			nested_map = nested_kv.ToMap()

			delete(k, key)
			k.Set(key, nested_map)
			continue
		}
	}
}

// Converts the key-valueto the golang map
func (k KeyValue) ToMap() map[string]interface{} {
	k.set_number()
	return map[string]interface{}(k)
}

// Returns the serialized key-value as a series of bytes
func (k KeyValue) ToBytes() ([]byte, error) {
	err := k.no_nil_value()
	if err != nil {
		return []byte{}, fmt.Errorf("nil value: %w", err)
	}
	k.set_number()

	bytes, err := json.Marshal(k)
	if err != nil {
		return []byte{}, fmt.Errorf("json.serialize: '%w'", err)
	}

	return bytes, nil
}

// Returns the serialized key-value as a string
func (k KeyValue) ToString() (string, error) {
	bytes, err := k.ToBytes()
	if err != nil {
		return "", fmt.Errorf("k.ToBytes %v: %w", k, err)
	}

	return string(bytes), nil
}

// Returns the key-value as an interface
func (k KeyValue) ToInterface(i interface{}) error {
	bytes, err := k.ToBytes()
	if err != nil {
		return fmt.Errorf("k.ToBytes of %v: '%w'", k, err)
	}
	err = json.Unmarshal(bytes, i)
	if err != nil {
		return fmt.Errorf("json.deserialize(%s to %T): '%w'", bytes, i, err)
	}

	return nil
}

// Add a new parameter
func (k KeyValue) Set(name string, value interface{}) KeyValue {
	k[name] = value

	return k
}

func (parameters KeyValue) exist(name string) error {
	_, exists := parameters[name]
	if !exists {
		return fmt.Errorf("'%s' not found in %v", name, parameters)
	}

	return nil
}

// Returns the parameter as an uint64
func (parameters KeyValue) GetUint64(name string) (uint64, error) {
	if err := parameters.exist(name); err != nil {
		return 0, fmt.Errorf("exist: %w", err)
	}
	raw := parameters[name]

	pure_value, ok := raw.(uint64)
	if ok {
		return pure_value, nil
	}
	value, ok := raw.(float64)
	if ok {
		return uint64(value), nil
	}

	json_value, ok := raw.(json.Number)
	if ok {
		number, err := strconv.ParseUint(string(json_value), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("strconv.ParseUint(%v (type %T) as json number %v): '%w'", raw, raw, json_value, err)
		}
		return number, nil
	}

	string_value, ok := raw.(string)
	if !ok {
		return 0, fmt.Errorf("'%s' parameter type %T, can not convert to number", name, raw)
	}
	number, err := strconv.ParseUint(string_value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("strconv.ParseUint string %v (original: %v): '%w'", string_value, raw, err)
	}

	return number, nil
}

func (parameters KeyValue) GetFloat64(name string) (float64, error) {
	if err := parameters.exist(name); err != nil {
		return 0, fmt.Errorf("exist: %w", err)

	}
	raw := parameters[name]

	pure_value, ok := raw.(float64)
	if ok {
		return pure_value, nil
	}
	value, ok := raw.(json.Number)
	if ok {
		v, err := value.Float64()
		if err != nil {
			return 0, fmt.Errorf("json.Number.Float64() of %v (original: %v): '%w'", value, raw, err)
		}
		return v, nil
	}
	string_value, ok := raw.(string)
	if !ok {
		return 0, fmt.Errorf("'%s' parameter type %T, can not convert to number", name, raw)
	}
	number, err := strconv.ParseFloat(string(string_value), 64)
	if err != nil {
		return 0, fmt.Errorf("strconv.ParseUint string %v (original: %v): '%w'", string_value, raw, err)
	}

	return number, nil
}

func (parameters KeyValue) GetBoolean(name string) (bool, error) {
	if err := parameters.exist(name); err != nil {
		return false, fmt.Errorf("exist: %w", err)
	}
	raw := parameters[name]

	pure_value, ok := raw.(bool)
	if ok {
		return pure_value, nil
	}

	return false, fmt.Errorf("'%s' parameter type %T, can not convert to boolean", name, raw)
}

// Returns the parsed large number. If the number size is more than 64 bits.
func (parameters KeyValue) GetBigNumber(name string) (*big.Int, error) {
	if err := parameters.exist(name); err != nil {
		return nil, fmt.Errorf("exist: %w", err)
	}
	raw := parameters[name]

	value, ok := raw.(json.Number)
	if !ok {
		return nil, fmt.Errorf("json.Number: '%s' parameter type %T", name, raw)
	}

	number, ok := math.ParseBig256(string(value))
	if !ok {
		return nil, fmt.Errorf("math.ParseBig256 failed to parse %s from '%s'", name, value)
	}

	return number, nil
}

// Returns the paramater as a string
func (parameters KeyValue) GetString(name string) (string, error) {
	if err := parameters.exist(name); err != nil {
		return "", fmt.Errorf("exist: %w", err)
	}
	raw := parameters[name]

	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s parameter type %T, can not convert to string", name, raw)
	}

	return value, nil
}

// Returns list of strings
func (parameters KeyValue) GetStringList(name string) ([]string, error) {
	if err := parameters.exist(name); err != nil {
		return nil, fmt.Errorf("exist: '%w'", err)
	}
	raw := parameters[name]

	values, ok := raw.([]interface{})
	if !ok {
		ready_list, ok := raw.([]string)
		if !ok {
			return nil, fmt.Errorf("'%s' parameter type %T, can not convert to string list", name, raw)
		} else {
			return ready_list, nil
		}
	}

	list := make([]string, len(values))
	for i, raw_value := range values {
		v, ok := raw_value.(string)
		if !ok {
			return nil, fmt.Errorf("parameter %s[%d] type is %T, can not convert to string %v", name, i, raw_value, raw_value)
		}

		list[i] = v
	}

	return list, nil
}

// Returns the parameter as a slice of map:
//
// []key_value.KeyValue
func (parameters KeyValue) GetKeyValueList(name string) ([]KeyValue, error) {
	if err := parameters.exist(name); err != nil {
		return nil, fmt.Errorf("exist: %w", err)
	}
	raw := parameters[name]

	values, ok := raw.([]interface{})
	if !ok {
		ready_list, ok := raw.([]KeyValue)
		if !ok {
			return nil, fmt.Errorf("'%s' parameter type %T, can not convert to key-value list", name, raw)
		} else {
			return ready_list, nil
		}
	}

	list := make([]KeyValue, len(values))
	for i, raw_value := range values {
		v, ok := raw_value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("parameter %s[%d] type is %T, can not convert to key-value %v", name, i, raw_value, raw_value)
		}

		list[i] = New(v)
	}

	return list, nil
}

// Returns the parameter as a map:
//
// key_value.KeyValue
func (parameters KeyValue) GetKeyValue(name string) (KeyValue, error) {
	if err := parameters.exist(name); err != nil {
		return nil, fmt.Errorf("exist: %w", err)
	}
	raw := parameters[name]

	value, ok := raw.(KeyValue)
	if ok {
		return value, nil
	}

	raw_map, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'%s' parameter type %T, can not convert to key-value", name, raw)
	}

	return New(raw_map), nil
}
