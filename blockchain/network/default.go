package network

// Returns the default configuration for the service
// Returns the list of default configurations
func DefaultConfiguration() string {
	networks := `
	[
		{"id": "56", "providers": [
			{
				"url": "https://rpc.ankr.com/bsc",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "1", "providers": [
			{
				"url": "https://eth.llamarpc.com",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "1285", "providers": [
			{
				"url": "https://moonriver.unitedbloc.com:2000",
				"length": 1000
			},
			{
				"url": "https://moonriver.api.onfinality.io/public",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "1284", "providers": [
			{
				"url": "https://1rpc.io/glmr",
				"length": 1000
			},
			{
				"url": "https://rpc.ankr.com/moonbeam",
				"length": 1000
			}
		], "type": "evm"},
		{"id": "imx", "providers": [
			{
				"url": "https://api.sandbox.x.immutable.com/",
				"length": 1000
			}
		], "type": "imx"}
	]`

	return networks
}
