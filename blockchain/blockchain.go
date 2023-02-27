package blockchain

import (
	"fmt"

	evm_client "github.com/blocklords/gosds/blockchain/evm/client"
	evm_worker "github.com/blocklords/gosds/blockchain/evm/worker"
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
			new_client, err := evm_client.New(new_network)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create EVM client: %v", err)
			}

			new_worker := evm_worker.New(new_client, nil, false)
			go new_worker.Sync()
			// } else if new_network.Type == network.IMX {
			// new_client, err := imx_client.New(new_network)
			// if err != nil {
			// return fmt.Errorf("gosds/blockchain: failed to create IMX client: %v", err)
			// }

			// new_worker := imx_worker.New()
		}
	}

	return nil
}
