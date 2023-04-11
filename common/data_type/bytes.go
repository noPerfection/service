// Package data_type defines the generic data types used in SDS.
//
// Supported data types are:
//   - Queue is the list where the new element is added to the end,
//     but when element is taken its taken from the top.
//     Queue doesn't allow addition of any kind of element. All elements should have the same type.
//   - key_value different kind of maps
//   - serialize functions to serialize any structure to the bytes and vice versa.
package data_type

// SerializeBytes turns the bytes into base64 string with the "==" tail.
//
// JSON marshalling passes the bytes array as base64 without the tail.
// These makes it hard to differentiate base64 from other types of string
// to unmarshal bytes.
//
// Returns the base64 string with the "==" tail on success.
// On failure, the method returns an empty string.
//
// Use DeserializeBytes() to deserialize base64 string into bytes array.
func SerializeBytes(bytes []byte) string {
	final_bytes, err := Serialize(bytes)
	if err != nil {
		return ""
	}

	base_string := string(final_bytes) + "=="
	return base_string
}

// DeserializeBytes turns base64 string with "==" tail into
// the sequences of the bytes.
//
// It's the reverse of SerializeBytes() function.
//
// If the string is not a valid base64 encoded string with "==" tail
// then function returns an empty sequence of the bytes.
func DeserializeBytes(str string) []byte {
	if len(str) == 0 {
		return []byte{}
	}

	if len(str) < 2 || str[len(str)-2:] != "==" {
		return []byte{}
	}

	var bytes []byte
	err := Deserialize([]byte(str[:len(str)-2]), &bytes)
	if err != nil {
		return []byte{}
	}

	return bytes
}
