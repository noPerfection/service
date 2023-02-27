package blockchain

import (
	"fmt"

	"github.com/blocklords/gosds/app/configuration"
	evm_categorizer "github.com/blocklords/gosds/blockchain/evm/categorizer"
	evm_client "github.com/blocklords/gosds/blockchain/evm/client"
	imx_categorizer "github.com/blocklords/gosds/blockchain/imx/categorizer"
	imx_client "github.com/blocklords/gosds/blockchain/imx/client"

	evm_worker "github.com/blocklords/gosds/blockchain/evm/worker"
	"github.com/blocklords/gosds/blockchain/imx"
	imx_worker "github.com/blocklords/gosds/blockchain/imx/worker"

	"github.com/blocklords/gosds/blockchain/network"

	zmq "github.com/pebbe/zmq4"
)

// Start the workers
func StartWorkers(app_config *configuration.Config) error {
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

			// Categorizer of the smartcontracts
			// This categorizers are interacting with the SDS Categorizer
			categorizer := evm_categorizer.NewManager(new_network)
			go categorizer.Start()
		} else if new_network.Type == network.IMX {
			err := imx.ValidateEnv(app_config)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to validate IMX specific config: %v", err)
			}

			new_client, err := imx_client.New(new_network)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create IMX client: %v", err)
			}

			new_worker := imx_worker.New(new_client, nil, false)
			go new_worker.Sync()

			imx_manager := imx_categorizer.NewManager(app_config, new_network)
			go imx_manager.Start()
		}
	}

	return nil
}

func NewCategorizerPusher(network_id string) (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, err
	}

	url := "cat_" + network_id
	if err := sock.Bind("inproc://" + url); err != nil {
		return nil, fmt.Errorf("trying to create categorizer for network id %s: %v", network_id, err)
	}

	return sock, nil
}
