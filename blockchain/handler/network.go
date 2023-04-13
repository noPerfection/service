package handler

import (
	"github.com/blocklords/sds/blockchain/network"
)

// GetNetworkRequest defines the required
// parameters in message.Request.Parameters for
// NETWORK_COMMAND
type GetNetworkRequest struct {
	NetworkId   string              `json:"network_id"`
	NetworkType network.NetworkType `json:"network_type"`
}

// GetNetworkReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling NETWORK_COMMAND
type GetNetworkReply = network.Network

// GetNetworksRequest defines the required
// parameters in message.Request.Parameters for
// NETWORKS_COMMAND
type GetNetworksRequest struct {
	NetworkType network.NetworkType
}

// GetNetworksReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling NETWORKS_COMMAND
type GetNetworksReply struct {
	Networks network.Networks `json:"networks"`
}

// GetNetworkIdsReply defines the required
// parameters in message.Request.Parameters for
// NETWORK_IDS_COMMAND
type GetNetworkIdsRequest struct {
	NetworkType network.NetworkType
}

// GetNetworksReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling NETWORK_IDS_COMMAND
type GetNetworkIdsReply struct {
	NetworkIds []string `json:"network_ids"`
}
