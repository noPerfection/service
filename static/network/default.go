package network

// Returns the default configuration for the service
// Returns the list of default configurations
func DefaultConfiguration() string {
	networks := `
	[
		{"id": "56", "provider": "https://rpc.ankr.com/bsc", "flag": 1},
		{"id": "1", "provider": "https://eth.llamarpc.com", "flag": 1},
		{"id": "1285", "provider": "https://moonriver.public.blastapi.io", "flag": 1},
		{"id": "1284", "provider": "https://1rpc.io/glmr", "flag": 1},
		{"id": "imx", "provider": "https://api.sandbox.x.immutable.com/", "flag": 2}
	]`

	return networks
}
