/*
The environment package's file category handles loading

.env or any other environment variable that is provided by the user
*/
package env

import (
	"os"
	"strconv"
)

// Checks whether the environment variable exists or not
func Exists(name string) bool {
	_, exists := os.LookupEnv(name)
	return exists
}

// Returns an environment variable as a string
func GetString(name string) string {
	return os.Getenv(name)
}

// Returns an environment variable as a number
func GetNumeric(name string) uint {
	value := os.Getenv(name)
	if len(value) == 0 {
		return 0
	}

	num, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}

	return uint(num)
}
