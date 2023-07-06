// Package env was created for one purpose only: LoadAnyEnv
package env

import (
	"fmt"

	"github.com/Seascape-Foundation/sds-service-lib/configuration/argument"
	"github.com/joho/godotenv"
)

// LoadAnyEnv gets the list of all .env file paths in the command line argument.
// Then loads them up to the application's environment variables.
//
// The values later will be available via app/configuration.Config.
func LoadAnyEnv() error {
	opts := argument.GetEnvPaths()

	if len(opts) == 0 {
		return nil
	}

	err := godotenv.Load(opts...)
	if err != nil {
		return fmt.Errorf("godotenv.Load for paths %v: %w", opts, err)
	}
	return nil
}
