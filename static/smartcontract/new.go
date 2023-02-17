package smartcontract

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Creates a new smartcontract from the JSON
func New(parameters key_value.KeyValue) (*Smartcontract, error) {
	network_id, err := parameters.GetString("network_id")
	if err != nil {
		return nil, err
	}
	address, err := parameters.GetString("address")
	if err != nil {
		return nil, err
	}
	abi_hash, err := parameters.GetString("abi_hash")
	if err != nil {
		return nil, err
	}
	txid, err := parameters.GetString("txid")
	if err != nil {
		return nil, err
	}
	// optional parameters
	deployer, err := parameters.GetString("deployer")
	if err != nil {
		deployer = ""
	}
	pre_deploy_block_number, err := parameters.GetUint64("pre_deploy_block_number")
	if err != nil {
		pre_deploy_block_number = 0
	}
	pre_deploy_block_timestamp, err := parameters.GetUint64("pre_deploy_block_timestamp")
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
