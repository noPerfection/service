package handler

import (
	"github.com/blocklords/sds/blockchain/network"
)

type GetNetworkRequest struct {
	NetworkId   string              `json:"network_id"`
	NetworkType network.NetworkType `json:"network_type"`
}
type GetNetworkReply = network.Network

type GetNetworksRequest = network.NetworkType
type GetNetworksReply = network.Networks

type GetNetworkIdsRequest = network.NetworkType
type GetNetworkIdsReply = []string
