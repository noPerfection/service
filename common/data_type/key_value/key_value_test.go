package key_value

import (
	"testing"

	"encoding/json"

	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestKeyValueSuite struct {
	suite.Suite
	key KeyValue
}

// Setup
// Setup checks the New() functions
// Setup checks ToMap() functions
func (suite *TestKeyValueSuite) SetupTest() {
	empty := map[string]interface{}{}
	kv := New(empty)
	suite.Require().EqualValues(empty, kv)
	empty_kv := Empty()
	suite.Require().EqualValues(kv, empty_kv)
	suite.Require().Equal(empty, kv.ToMap())
	suite.Require().Equal(empty, empty_kv.ToMap())

	// no null value could be used
	invalid_str := `{"param_1":null,"param_2":"string_value","param_3":{"nested_1":5,"nested_2":"hello"}}`
	_, err := NewFromString(invalid_str)
	suite.Require().Error(err)

	// no null value could be used in the nested values
	invalid_str = `{"param_1":1,"param_2":"string_value","param_3":{"nested_1":5,"nested_2":null}}`
	_, err = NewFromString(invalid_str)
	suite.Require().Error(err)

	// validate the parameters
	str := `{"param_1":2,"param_2":"string_value","param_3":{"nested_1":5,"nested_2":"hello"}}`
	str_kv, err := NewFromString(str)
	suite.Require().NoError(err)
	map_key := str_kv.ToMap()

	var num_2 json.Number = json.Number("2")
	var num_5 json.Number = json.Number("5")

	str_map := map[string]interface{}{
		"param_1": num_2,
		"param_2": "string_value",
		"param_3": map[string]interface{}{
			"nested_1": num_5,
			"nested_2": "hello",
		},
	}
	invalid_map := map[string]interface{}{
		"param_1": 2,
		"param_2": "string_value",
		"param_3": map[string]interface{}{
			"nested_1": uint64(5),
			"nested_2": "hello",
		},
	}

	// one of the parameters is not uint64
	suite.Require().NotEqual(invalid_map, map_key)
	suite.Require().Equal(str_map, map_key)

	type Nested struct {
		Nested1 uint64 `json:"nested_1"`
		Nested2 string `json:"nested_2"`
	}
	type Temp struct {
		Param1 uint64 `json:"param_1"`
		Param2 string `json:"param_2"`
		Param3 Nested `json:"param_3"`
	}
	new_temp := Temp{
		Param1: uint64(2),
		Param2: "string_value",
		Param3: Nested{
			Nested1: uint64(5),
			Nested2: "hello",
		},
	}
	interface_kv, err := NewFromInterface(new_temp)
	suite.Require().NoError(err)
	// The number type in the kv is json.Number
	// But in the temp its not
	suite.Require().NotEqual(str_kv, interface_kv)
	suite.Require().EqualValues(map_key, interface_kv.ToMap())

	// invalid, the parameters are as is in the struct
	// it misses `json:<param>`
	type InvalidTemp struct {
		Param1 uint64
		Param2 string `json:"param_2"`
		Param3 Nested `json:"param_3"`
	}
	invalid_temp := InvalidTemp{
		Param1: uint64(2),
		Param2: "string_value",
		Param3: Nested{
			Nested1: uint64(5),
			Nested2: "hello",
		},
	}
	interface_kv, err = NewFromInterface(invalid_temp)
	suite.Require().NoError(err)
	suite.Require().NotEqual(str_kv, interface_kv)

	// Any number is returned as a uint64
	type TempUint struct {
		Param1 uint   `json:"param_1"`
		Param2 string `json:"param_2"`
		Param3 Nested `json:"param_3"`
	}
	uint_temp := TempUint{
		Param1: uint(2),
		Param2: "string_value",
		Param3: Nested{
			Nested1: uint64(5),
			Nested2: "hello",
		},
	}
	interface_kv, err = NewFromInterface(uint_temp)
	suite.Require().NoError(err)
	param_1, err := interface_kv.GetUint64("param_1")
	suite.Require().NoError(err)
	suite.Require().Equal(uint64(2), param_1)

	suite.key = str_kv
}

func (suite *TestKeyValueSuite) TestToString() {
	str := `{"param_1":2,"param_2":"string_value","param_3":{"nested_1":5,"nested_2":"hello"}}`
	kv_str, err := suite.key.ToString()
	suite.Require().NoError(err)
	suite.Require().Equal(str, kv_str)

	nil_kv := KeyValue(map[string]interface{}{"nil_param": nil})
	_, err = nil_kv.ToString()
	suite.Require().Error(err)

	// Empty parameter is okay
	empty_param := KeyValue(map[string]interface{}{"empty_param": ""})
	_, err = empty_param.ToString()
	suite.Require().NoError(err)
}

func (suite *TestKeyValueSuite) TestToInterface() {
	type Nested struct {
		Nested1 uint64 `json:"nested_1"`
		Nested2 string `json:"nested_2"`
	}
	type Temp struct {
		Param1 uint64 `json:"param_1"`
		Param2 string `json:"param_2"`
		Param3 Nested `json:"param_3"`
	}
	var new_temp Temp
	err := suite.key.ToInterface(&new_temp)
	suite.Require().NoError(err)

	// Can not convert to the scalar format
	// But it will be empty
	// since its not passed by a pointer
	var invalid_temp string
	err = suite.key.ToInterface(invalid_temp)
	suite.Require().Error(err)

	// Can convert with the wrong type
	// But check it in the struct
	type InvalidTemp struct {
		Param1 uint64
		Param2 string `json:"param_2"`
		Param3 Nested `json:"param_3"`
	}
	var no_json_temp InvalidTemp
	err = suite.key.ToInterface(&no_json_temp)
	suite.Require().NoError(err)

	// Can convert to another type
	// with the invalid parameter type.
	// The map's param_2 is a string.
	type InvalidType struct {
		Param1 uint64 `json:"param_1"`
		Param2 uint64 `json:"param_2"`
		Param3 Nested `json:"param_3"`
	}
	var has_more_temp InvalidType
	err = suite.key.ToInterface(&has_more_temp)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestKeyValue(t *testing.T) {
	suite.Run(t, new(TestKeyValueSuite))
}
