package provider

const (
	// PROVIDER_MAX_LENGTH defines the limit.
	// When the provider is set in the configuration the "length" parameter
	// value should not exceed this limit
	PROVIDER_MAX_LENGTH uint64 = 10_000
)

// Provider is the Url wrapper to the remote
// blockchain node along with the Url parameters.
//
// The Provider is not responsible for connecting.
// Refer to blockchain/<blockchain type>/client
type Provider struct {
	Url    string `json:"url"`
	Length uint64 `json:"length"` // How many blocks we can fetch from this provider
}
