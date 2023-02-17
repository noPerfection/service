// The network package is used to get the blockchain network information.
package network

type Network struct {
	Id       string `json:"id"`
	Provider string `json:"provider"`
	Flag     int8   `json:"flag"` // With VM or Without VM
}

// Whether the network with network_id exists in the networks list
func (networks Networks) Exist(network_id string) bool {
	for _, network := range networks {
		if network.Id == network_id {
			return true
		}
	}

	return false
}
