// Package arg is used to read command line arguments of the application.
package arg

import (
	"fmt"
	"github.com/ahmetson/service-lib/os/path"
	"os"
	"strings"
)

// List of service flags
const (
	Secure             = "secure"       // If passed, then TCP sockets will require authentication. Default is false
	BuildConfiguration = "build-config" // returns the extensions, controllers that the service will need
	Path               = "path"         // The file path to include for generation it should end with .yml
	Url                = "url"          // The url of the service to set when building the config
	Configuration      = "config"       // The path to the config. It should end with .yml
)

// GetEnvPaths any command line data that comes after the files are .env file paths
// Any arg for application without '--' prefix is considered to be path to the
// environment file.
func GetEnvPaths() ([]string, error) {
	args := os.Args[1:]
	if len(args) == 0 {
		return []string{}, nil
	}

	execPath, err := path.GetExecPath()
	if err != nil {
		return []string{}, fmt.Errorf("path.GetExecPath: %w", err)
	}
	paths := make([]string, 0)

	for _, arg := range args {
		if len(arg) < 4 {
			continue
		}

		lastPart := arg[len(arg)-4:]
		if lastPart != ".env" {
			continue
		}

		if arg[:2] == "--" {
			continue
		}

		paths = append(paths, path.GetPath(execPath, arg))
	}

	return paths, nil
}

// GetArguments Load arguments, not the environment variable paths.
// Arguments starts with '--'
func GetArguments() []string {
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

// Exist This function is same as `env.HasArgument`,
// except `env.ArgumentExist()` loads arguments automatically.
func Exist(argument string) bool {
	return Has(GetArguments(), argument)
}

// ExtractValue Extracts the value of the arg if it has.
// The arg value comes after "=".
//
// This function gets the arguments from the CLI automatically.
//
// If the arg doesn't exist, then returns an empty string.
// Therefore, you should check for the arg existence by calling `arg.Exist()`
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

	value, err := getValue(found)
	if err != nil {
		return "", fmt.Errorf("getValue for %s arg: %w", required, err)
	}

	return value, nil
}

// Value Extracts the value of the arg if it's exists.
// Similar to getValue() but doesn't accept the
func Value(name string) (string, error) {
	return ExtractValue(GetArguments(), name)
}

// getValue Extracts the value of the arg.
// Argument comes after '='
func getValue(argument string) (string, error) {
	parts := strings.Split(argument, "=")
	if len(parts) != 2 {
		return "", fmt.Errorf("strings.split(`%s`) should has two parts", argument)
	}

	if len(parts[1]) == 0 {
		return "", fmt.Errorf("value of --%s is empty", argument)
	}
	return parts[1], nil
}

// Has checks is the required arg exists among arguments or not.
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
