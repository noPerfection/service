// Generic Type package handles the common functions of multiple SDS Data structures.
package data_type

import (
	"github.com/blocklords/sds/app/account"
	categorizer_log "github.com/blocklords/sds/categorizer/event"
	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/key_value"

	static_configuration "github.com/blocklords/sds/static/configuration"
	static_smartcontract "github.com/blocklords/sds/static/smartcontract"
	static_smartcontract_key "github.com/blocklords/sds/static/smartcontract/key"
)

type List interface {
	*categorizer_log.Log | *categorizer_smartcontract.Smartcontract |
		*static_configuration.Configuration | *static_smartcontract.Smartcontract |
		*account.Account
}

type StringList interface {
	*static_smartcontract_key.Key
}

func ToMapList[V List](list []V) []map[string]interface{} {
	map_list := make([]map[string]interface{}, len(list))
	for i, element := range list {
		kv, _ := key_value.NewFromInterface(element)
		map_list[i] = kv.ToMap()
	}

	return map_list
}

// Converts the data structs to the list of strings.
// []string
func ToStringList[V StringList](list []V) []string {
	string_list := make([]string, len(list))
	for i, element := range list {
		string_list[i] = string(*element)
	}

	return string_list
}
