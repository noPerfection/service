// Package parameter is the utility function that keeps the client-controller
// request parameters.
//
// Supported request parameters:
// - REQUEST_TIMEOUT how long the client waits until the controller doesn't reply.
// - ATTEMPT how many attempts the client makes to send the Request before it returns an error.
package parameter

import (
	"context"
	"time"

	"github.com/blocklords/sds/app/configuration"
)

// todo remove the functions
// and set the default values to the request timeout

// Request-Reply checks the internet connection after this amount of time.
// This is the default time if argument wasn't given that changes the REQUEST_TIMEOUT
const (
	// Timeout in the seconds.
	// Set the SDS_REQUEST_TIMEOUT.
	// If the timeout is float, then its rounded
	REQUEST_TIMEOUT = 30 * time.Second
	//
	// How many attempts to do if the remote socket doesn't responds.
	// Set the SDS_REQUEST_ATTEMPT
	// If the SDS_REQUEST_ATTEMPT is a float number
	// then its rounded.
	ATTEMPT = uint(5)
)

// NewTimeoutContext returns a new context with the request timeout
// and the timeout with SDS_REQUEST_TIMEOUT
func NewContextWithTimeout(parent context.Context, app_config *configuration.Config) (context.Context, context.CancelFunc) {
	new_ctx, cancel_func := context.WithTimeout(parent, RequestTimeout(app_config))
	return new_ctx, cancel_func
}

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
