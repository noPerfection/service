// Package handler defines the commands that this service or
// sub services are handling.
// Besides the command names it also defines the data types for requesting/replying the commands.
//
// Defining the data types means validating them over the message.
package handler

import (
	"github.com/blocklords/sds/common/blockchain"
)

// RecentBlockHeaderRequest is the message.Request.Parameters for
// RECENT_BLOCK_NUMBER command.
type RecentBlockHeaderRequest = blockchain.BlockHeader

// RecentBlockHeaderReply is the message.Reply.Parameters that is
// replied by RECENT_BLOCK_NUMBER command handler.
type RecentBlockHeaderReply = blockchain.BlockHeader
