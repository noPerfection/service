// Package parameter is the utility function that keeps the client-server
// request parameters.
//
// Supported request parameters:
// - REQUEST_TIMEOUT how long the client waits until the server doesn't reply.
// - ATTEMPT how many attempts the client makes to send the Request before it returns an error.
package parameter

import (
	"context"
	"time"

	"github.com/ahmetson/service-lib/config"
)

// todo remove the functions
// and set the default values to the request timeout

// Request-Reply checks the internet connection after this amount of time.
// This is the default time if argument wasn't given that changes the REQUEST_TIMEOUT
const (
	// DefaultTimeout in the seconds.
	// Set the SDS_REQUEST_TIMEOUT.
	// If the timeout is float, then its rounded
	DefaultTimeout = 30 * time.Second
	// DefaultAttempt How many attempts to do if the client socket doesn't respond.
	// Set the SDS_REQUEST_ATTEMPT
	// If the SDS_REQUEST_ATTEMPT is a float number
	// then its rounded.
	DefaultAttempt = uint(5)
)

// NewContextWithTimeout returns a new orchester with the request timeout
// and the timeout with SDS_REQUEST_TIMEOUT
func NewContextWithTimeout(parent context.Context, appConfig *config.Config) (context.Context, context.CancelFunc) {
	newCtx, cancelFunc := context.WithTimeout(parent, RequestTimeout(appConfig))
	return newCtx, cancelFunc
}

// RequestTimeout Request timeout, from the config.
// If the config doesn't exist, then return the default value.
func RequestTimeout(appConfig *config.Config) time.Duration {
	requestTimeout := DefaultTimeout
	if appConfig != nil && appConfig.Exist("SDS_REQUEST_TIMEOUT") {
		envTimeout := appConfig.GetUint64("SDS_REQUEST_TIMEOUT")
		if envTimeout != 0 {
			requestTimeout = time.Duration(envTimeout) * time.Second
		}
	}

	return requestTimeout
}

// Attempt How many attempts we make to request the client service before we will
// return an error.
// It returns the attempt amount from the config.
// If the config doesn't exist, then we the default value.
func Attempt(appConfig *config.Config) uint {
	attempt := DefaultAttempt
	if appConfig != nil && appConfig.Exist("SDS_REQUEST_ATTEMPT") {
		envAttempt := appConfig.GetUint64("SDS_REQUEST_ATTEMPT")
		if envAttempt != 0 {
			attempt = uint(envAttempt)
		}
	}

	return attempt
}
