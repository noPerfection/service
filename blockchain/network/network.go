// The network package is used to get the blockchain network information.
package network

import (
	"fmt"

	"github.com/blocklords/sds/blockchain/network/provider"
)

type Network struct {
	Id        string              `json:"id"`
	Providers []provider.Provider `json:"providers"`
	Type      NetworkType         `json:"type"` // With VM or Without VM
}

// Returns the provider url
func (n *Network) GetFirstProviderUrl() (string, error) {
	if len(n.Providers) == 0 {
		return "", fmt.Errorf("there is no providers")
	}
	return n.Providers[0].Url, nil
}

// Returns the block range length that is available for the first provider
func (n *Network) GetFirstProviderLength() (uint64, error) {
	if len(n.Providers) == 0 {
		return 0, fmt.Errorf("there is no providers")
	}
	return n.Providers[0].Length, nil
}
