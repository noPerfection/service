package parameter

import (
	"time"

	"github.com/blocklords/sds/app/configuration"
)

// todo remove the functions
// and set the default values to the request timeout

// Request-Reply checks the internet connection after this amount of time.
// This is the default time if argument wasn't given that changes the REQUEST_TIMEOUT
const (
	REQUEST_TIMEOUT = 30 * time.Second //  msecs, (> 1000!)
	ATTEMPT         = uint(5)
)

// Request timeout, from the configuration.
// If the configuration doesn't exist, then return the default value.
func RequestTimeout(app_config *configuration.Config) time.Duration {
	request_timeout := REQUEST_TIMEOUT
	if app_config.Exist("SDS_REQUEST_TIMEOUT") {
		env_timeout := app_config.GetUint64("SDS_REQUEST_TIMEOUT")
		if env_timeout != 0 {
			request_timeout = time.Duration(env_timeout) * time.Second
		}
	}

	return request_timeout
}

// How many attempts we make to request the remote service before we will
// return an error.
// It returns the attempt amount from the configuration.
// If the configuration doesn't exist, then we the default value.
func Attempt(app_config *configuration.Config) uint {
	attempt := ATTEMPT
	if app_config.Exist("SDS_REQUEST_ATTEMPT") {
		env_attempt := app_config.GetUint64("SDS_REQUEST_ATTEMPT")
		if env_attempt != 0 {
			attempt = uint(env_attempt)
		}
	}

	return attempt
}
