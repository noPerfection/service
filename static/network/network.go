// The network package is used to get the blockchain network information.
package network

type Network struct {
	Id       string
	Provider string
	Flag     int8 // With VM or Without VM
}

func (n *Network) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"id":       n.Id,
		"provider": n.Provider,
		"flag":     n.Flag,
	}
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
