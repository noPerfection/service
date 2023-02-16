package smartcontract

import (
	"encoding/json"

	"github.com/blocklords/gosds/static/smartcontract/key"
)

type Smartcontract struct {
	// Body abi.ABI
	NetworkId               string
	Address                 string
	AbiHash                 string
	Txid                    string
	Deployer                string
	PreDeployBlockNumber    int
	PreDeployBlockTimestamp int
	exists                  bool
}

func (c *Smartcontract) Key() key.Key {
	return key.New(c.NetworkId, c.Address)
}

func (c *Smartcontract) SetExists(exists bool) {
	c.exists = exists
}

// JSON represantion of the static.Smartcontract
func (smartcontract *Smartcontract) ToJSON() map[string]interface{} {
	i := map[string]interface{}{}
	i["network_id"] = smartcontract.NetworkId
	i["address"] = smartcontract.Address
	i["abi_hash"] = smartcontract.AbiHash
	i["txid"] = smartcontract.Txid
	i["pre_deploy_block_number"] = smartcontract.PreDeployBlockNumber
	i["pre_deploy_block_timestamp"] = smartcontract.PreDeployBlockTimestamp
	i["deployer"] = smartcontract.Deployer

	return i
}

// The JSON string represantion of the static.Smartcontract
func (smartcontract *Smartcontract) ToString() string {
	s := smartcontract.ToJSON()
	byt, err := json.Marshal(s)
	if err != nil {
		return ""
	}

	return string(byt)
}
