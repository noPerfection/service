// Package database keeps the utility
// functions that converts
// database type to
// internal golang type
package database

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Returns the type of database type
// that matches to the golang type
//
// If the data type wasn't detected, then
// it returns an empty result.
func detect_type(database_type *sql.ColumnType) string {
	switch database_type.DatabaseTypeName() {
	case "VARCHAR":
		return "string"
	case "JSON":
		return "[]byte"
	case "SMALLINT":
		return "int64"
	case "BIGINT":
		return "int64"
	case "UNSIGNED SMALLINT":
		return "uint64"
	case "UNSIGNED BIGINT":
		return "uint64"
	}
	return ""
}

// Set the value into kv KeyValue,
// but first converting it into the desired
// golang parameter
func SetValue(kv key_value.KeyValue, database_type *sql.ColumnType, raw interface{}) error {
	golang_type := detect_type(database_type)
	if golang_type == "" {
		return fmt.Errorf("unsupported database type %s", database_type.DatabaseTypeName())
	}

	switch golang_type {
	case "string":
		if raw == nil {
			kv.Set(database_type.Name(), "")
			return nil
		}
		value, ok := raw.(string)
		if !ok {
			bytes, ok := raw.([]byte)
			if !ok {
				return fmt.Errorf("couldn't convert %v of type %T into 'string'", raw, raw)
			}
			kv.Set(database_type.Name(), string(bytes))
			return nil
		}
		kv.Set(database_type.Name(), value)
		return nil
	case "[]byte":
		if raw == nil {
			kv.Set(database_type.Name(), []byte{})
			return nil
		}

		value, ok := raw.([]byte)
		if !ok {
			return fmt.Errorf("database value is expected to be '[]byte', but value %v of type %T", raw, raw)
		}
		kv.Set(database_type.Name(), value)
		return nil
	case "int64":
		if raw == nil {
			kv.Set(database_type.Name(), int64(0))
			return nil
		}
		value, ok := raw.(int64)
		if !ok {
			bytes, ok := raw.([]byte)
			if !ok {
				return fmt.Errorf("couldn't convert %v of type %T into 'int64'", raw, raw)
			}
			data, err := strconv.ParseInt(string(bytes), 10, 64)
			if err != nil {
				return fmt.Errorf("strconv.ParseInt: %w", err)
			}
			kv.Set(database_type.Name(), data)
			return nil
		}
		kv.Set(database_type.Name(), value)
		return nil
	case "uint64":
		if raw == nil {
			kv.Set(database_type.Name(), uint64(0))
			return nil
		}
		value, ok := raw.(uint64)
		if !ok {
			bytes, ok := raw.([]byte)
			if !ok {
				return fmt.Errorf("couldn't convert %v of type %T into 't converted into %s", raw, raw, golang_type)
			}
			data, err := strconv.ParseUint(string(bytes), 10, 64)
			if err != nil {
				return fmt.Errorf("strconv.ParseUint: %w", err)
			}
			kv.Set(database_type.Name(), data)
			return nil
		}
		kv.Set(database_type.Name(), value)
		return nil
	}

	return fmt.Errorf("no switch/case for setting value into KeyValue for %s field of %s type", database_type.Name(), golang_type)
}
