package blockchain

import (
	"fmt"

	"github.com/blocklords/gosds/blockchain/evm/client"
	"github.com/blocklords/gosds/blockchain/evm/worker"
	"github.com/blocklords/gosds/blockchain/network"
)

// Start the workers
func StartWorkers() error {
	networks, err := network.GetNetworks(network.ALL)
	if err != nil {
		return fmt.Errorf("gosds/blockchain: failed to get networks: %v", err)
	}

	for _, new_network := range networks {
		if new_network.Type == network.EVM {
			new_client, err := client.New(new_network)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create client: %v", err)
			}

			new_worker := worker.New(new_client, nil, false)
			go new_worker.Sync()
		}
	}

	return nil
}
