package parameter

import (
	"time"

	"github.com/blocklords/sds/app/configuration"
)

// Request-Reply checks the internet connection after this amount of time.
// This is the default time if argument wasn't given that changes the REQUEST_TIMEOUT
const (
	REQUEST_TIMEOUT = 30 * time.Second //  msecs, (> 1000!)
	ATTEMPT         = uint(5)
)

// Request timeout
func RequestTimeout() time.Duration {
	request_timeout := REQUEST_TIMEOUT
	app_config := configuration.New()
	if app_config.Exist("SDS_REQUEST_TIMEOUT") {
		env_timeout := app_config.GetUint64("SDS_REQUEST_TIMEOUT")
		if env_timeout != 0 {
			request_timeout = time.Duration(env_timeout) * time.Second
		}
	}

	return request_timeout
}

// How many attempts we make
func Attempt() uint {
	attempt := ATTEMPT
	app_config := configuration.New()
	if app_config.Exist("SDS_REQUEST_ATTEMPT") {
		env_attempt := app_config.GetUint64("SDS_REQUEST_ATTEMPT")
		if env_attempt != 0 {
			attempt = uint(env_attempt)
		}
	}

	return attempt
}
