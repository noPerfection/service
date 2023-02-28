package env

import (
	"github.com/blocklords/gosds/app/argument"
	"github.com/joho/godotenv"
)

// Load all .env files
func LoadAnyEnv() error {

	opts := argument.GetEnvPaths()

	godotenv.Load()

	if len(opts) > 0 {
		return godotenv.Load(opts...)
	}
	return nil
}
