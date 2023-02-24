// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package worker

import "github.com/blocklords/gosds/spaghetti/network_client"

type Workers map[string]*SpaghettiWorker

// Does the worker for the network id exist
func (workers Workers) Exist(network_id string) bool {
	_, ok := workers[network_id]
	return ok
}

// Return the client thats connected to the blockchain
func (workers Workers) Client(network_id string) *network_client.NetworkClient {
	return workers[network_id].client
}
