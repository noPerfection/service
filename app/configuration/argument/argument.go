// Application's command line arguments
package argument

import (
	"fmt"
	"os"
	"strings"

	"github.com/blocklords/sds/app/log"
)

const (
	PLAIN     = "plain"     // Switch off the authentication and encryption for SDS Service
	BROADCAST = "broadcast" // runs only broadcaster
	REPLY     = "reply"     // runs only request-reply server

	// network id, support only this network.
	// example:
	//    --network-id=5
	//
	//    support only network id 5
	NETWORK_ID = "network-id"

	// Set this arguments to print the logs
	BROADCAST_DEBUG = "broadcast-debug"
	SECURITY_DEBUG  = "security-debug"
)

// any command line data that comes after the files are .env file paths
// Any argument for application without '--' prefix is considered to be path to the
// environment file.
func GetEnvPaths() []string {
	args := os.Args[1:]
	if len(args) == 0 {
		return []string{}
	}

	paths := make([]string, 0)

	for _, arg := range args {
		if arg[:2] != "--" {
			paths = append(paths, arg)
		}
	}

	return paths
}

// Load arguments, not the environment variable paths.
// Arguments starts with '--'
func GetArguments(logger *log.Logger) []string {
	if logger != nil {
		env_logger := logger.Child("arguments", log.WITHOUT_REPORT_CALLER, log.WITHOUT_TIMESTAMP)

		env_logger.Info("To load .env files pass them as application arguments")
		env_logger.Info(".env in folder from where you call the app loaded automatically")
	}

	args := os.Args[1:]
	if len(args) == 0 {
		return []string{}
	}

	parameters := make([]string, 0)

	for _, arg := range args {
		if arg[:2] == "--" {
			parameters = append(parameters, arg[2:])
		}
	}

	return parameters
}

// This function is same as `env.HasArgument`,
// except `env.ArgumentExist()` loads arguments automatically.
func Exist(argument string) (bool, error) {
	return Has(GetArguments(nil), argument), nil
}

// Extracts the value of the argument if it has.
// The argument value comes after "=".
//
// This function gets the arguments from the CLI automatically.
//
// If the argument doesn't exist, then returns an empty string.
// Therefore you should check for the argument existence by calling `argument.Exist()`
func ExtractValue(arguments []string, required string) (string, error) {
	found := ""
	for _, argument := range arguments {
		// doesn't have a value
		if argument == required {
			continue
		}

		length := len(required)
		if len(argument) > length && argument[:length] == required {
			found = argument
			break
		}
	}

	value, err := GetValue(found)
	if err != nil {
		return "", fmt.Errorf("GetValue for %s argument: %w", required, err)
	}

	return value, nil
}

// Extracts the value of the argument.
// Argument comes after '='
func GetValue(argument string) (string, error) {
	parts := strings.Split(argument, "=")
	if len(parts) != 2 {
		return "", fmt.Errorf("strings.split(`%s`) should has two parts", argument)
	}

	return parts[1], nil
}

// Whehter the given argument exists or not.
func Has(arguments []string, required string) bool {
	for _, argument := range arguments {
		if argument == required {
			return true
		}

		length := len(required)
		if len(argument) > length && argument[:length] == required {
			return true
		}
	}

	return false
}
