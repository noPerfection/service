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

	paths := make([]string, 0)
	for _, opt := range opts {
		if len(opt) < 4 {
			continue
		}

		last_part := opt[len(opt)-4:]
		if last_part == ".env" {
			paths = append(paths, opt)
		}
	}

	if len(paths) == 0 {
		return nil
	}

	err := godotenv.Load(paths...)
	if err != nil {
		return fmt.Errorf("godotenv.Load for paths %v: %w", opts, err)
	}
	return nil
}
