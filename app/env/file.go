package env

import (
	"fmt"

	"github.com/blocklords/gosds/app/argument"
	"github.com/joho/godotenv"
)

// Load all .env files
func LoadAnyEnv() error {
	opts := argument.GetEnvPaths()

	godotenv.Load()

	if len(opts) > 0 {
		err := godotenv.Load(opts...)
		if err != nil {
			return fmt.Errorf("godotenv.Load for paths %v: %w", opts, err)
		}
	}
	return nil
}
