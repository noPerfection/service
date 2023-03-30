package env

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration/argument"
	"github.com/joho/godotenv"
)

// Load all *.env files
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
