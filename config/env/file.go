// Package env was created for one purpose only: LoadAnyEnv
package env

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"

	"github.com/ahmetson/service-lib/config/arg"
	"github.com/joho/godotenv"
)

// LoadAnyEnv gets the list of all .env file paths in the command line arg.
// Then loads them up to the application's environment variables.
//
// The values later will be available via app/config.Config.
//
// The .env files are locations are related to the exec path
func LoadAnyEnv() error {
	opts, err := arg.GetEnvPaths()
	if err != nil {
		return fmt.Errorf("arg.GetEnvPaths: %w", err)
	}

	if len(opts) == 0 {
		return nil
	}

	err = godotenv.Load(opts...)
	if err != nil {
		return fmt.Errorf("godotenv.Load for paths %v: %w", opts, err)
	}
	return nil
}

// WriteEnv writes the given key value to the file.
// If the file exists, then it will be truncated.
func WriteEnv(data key_value.KeyValue, path string) error {
	err := godotenv.Write(data.MapString(), path)
	if err != nil {
		return fmt.Errorf("godotenv.Write: %w", err)
	}

	return nil
}
