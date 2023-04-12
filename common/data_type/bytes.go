// Package data_type defines the generic data types used in SDS.
//
// Supported data types are:
//   - Queue is the list where the new element is added to the end,
//     but when element is taken its taken from the top.
//     Queue doesn't allow addition of any kind of element. All elements should have the same type.
//   - key_value different kind of maps
//   - serialize functions to serialize any structure to the bytes and vice versa.
package data_type

func AddJsonPrefix(bytes []byte) string {
	return "sds_json:" + string(bytes)
}

func IsJsonPrefixed(str string) bool {
	if len(str) < 9 {
		return false
	}

	return str[:9] == "sds_json:"
}

func DecodeJsonPrefixed(str string) string {
	if !IsJsonPrefixed(str) {
		return ""
	}

	unprefixed := str[9:]
	return unprefixed
}
