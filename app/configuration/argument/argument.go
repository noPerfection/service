// Package argument is used to read command line arguments of the application.
package argument

import (
	"fmt"
	"os"
	"strings"

	"github.com/blocklords/sds/app/log"
)

const (
	SECURE    = "secure"    // If passed, then TCP sockets will require authentication. Default is false
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
		if len(arg) < 4 {
			continue
		}

		last_part := arg[len(arg)-4:]
		if last_part != ".env" {
			continue
		}

		if arg[:2] == "--" {
			continue
		}
		paths = append(paths, arg)
	}

	return paths
}

// Load arguments, not the environment variable paths.
// Arguments starts with '--'
func GetArguments(parent *log.Logger) []string {
	logger, err := parent.Child("argument", log.WITH_TIMESTAMP)
	if err != nil {
		logger.Warn("parent.Child", "error", err)
		return []string{}
	}

	logger.Info("Supported app arguments",
		"--"+SECURE,
		"Enables security service",
		"--"+SECURITY_DEBUG,
		"To print the authentication logs",
	)

	args := os.Args[1:]
	if len(args) == 0 {
		logger.Info("No arguments were given")
		return []string{}
	}

	parameters := make([]string, 0)

	for _, arg := range args {
		if arg[:2] == "--" {
			parameters = append(parameters, arg[2:])
		}
	}

	logger.Info("All arguments read", "amount", len(parameters), "app parameters", parameters)

	return parameters
}

// This function is same as `env.HasArgument`,
// except `env.ArgumentExist()` loads arguments automatically.
func Exist(argument string) bool {
	return Has(GetArguments(nil), argument)
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
