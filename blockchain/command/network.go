package command

import (
	"github.com/blocklords/sds/blockchain/network"
)

type NetworkIds struct {
	NetworkType network.NetworkType `json:"network_type"`
}

type NetworkId struct {
	NetworkId   string              `json:"network_id"`
	NetworkType network.NetworkType `json:"network_type"`
}

type NetworkIdsReply struct {
	NetworkIds []string `json:"network_ids"`
}

type NetworkReply struct {
	Network network.Network `json:"network"`
}

type NetworksReply struct {
	Networks network.Networks `json:"networks"`
}
