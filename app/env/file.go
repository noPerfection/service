package env

import (
	"github.com/blocklords/gosds/app/argument"
	"github.com/joho/godotenv"
)

// Load all .env files
func LoadAnyEnv() error {
	opts, optErr := argument.GetEnvPaths()
	if optErr != nil {
		return optErr
	}

	godotenv.Load()

	if opts != nil {
		return godotenv.Load(opts...)
	}
	return nil
}
