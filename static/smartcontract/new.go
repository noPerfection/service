package smartcontract

import "github.com/blocklords/gosds/app/remote/message"

// Creates a new smartcontract from the JSON
func New(parameters map[string]interface{}) (*Smartcontract, error) {
	network_id, err := message.GetString(parameters, "network_id")
	if err != nil {
		return nil, err
	}
	address, err := message.GetString(parameters, "address")
	if err != nil {
		return nil, err
	}
	abi_hash, err := message.GetString(parameters, "abi_hash")
	if err != nil {
		return nil, err
	}
	txid, err := message.GetString(parameters, "txid")
	if err != nil {
		return nil, err
	}
	// optional parameters
	deployer, err := message.GetString(parameters, "deployer")
	if err != nil {
		deployer = ""
	}
	pre_deploy_block_number, err := message.GetUint64(parameters, "pre_deploy_block_number")
	if err != nil {
		pre_deploy_block_number = 0
	}
	pre_deploy_block_timestamp, err := message.GetUint64(parameters, "pre_deploy_block_timestamp")
	if err != nil {
		return nil, err
	}

	smartcontract := Smartcontract{
		exists:                  false,
		NetworkId:               network_id,
		Address:                 address,
		AbiHash:                 abi_hash,
		Txid:                    txid,
		Deployer:                deployer,
		PreDeployBlockNumber:    int(pre_deploy_block_number),
		PreDeployBlockTimestamp: int(pre_deploy_block_timestamp),
	}
	return &smartcontract, nil
}
