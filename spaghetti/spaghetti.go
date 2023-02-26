// The SDS Spaghetti module fetches the blockchain data and converts it into the internal format
// All other SDS Services are connecting to SDS Spaghetti.
//
// We have multiple workers.
// Atleast one worker for each network.
// This workers are called recent workers.
//
// Categorizer checks whether the cached block returned or not.
// If its a cached block, then switches to the block_range
package spaghetti

import (
	"fmt"

	"github.com/blocklords/gosds/spaghetti/log"
	"github.com/blocklords/gosds/spaghetti/network_client"
	"github.com/blocklords/gosds/spaghetti/worker"
	"github.com/blocklords/gosds/static/network"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/broadcast"
	"github.com/blocklords/gosds/app/controller"
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"
)

var static_socket *remote.Socket
var workers worker.Workers

// Run EVM blockchain clients
func setup_evm_workers(networks network.Networks, broadcast_channel chan message.Broadcast, debug bool) (map[string]*worker.SpaghettiWorker, error) {
	workers := make(worker.Workers, 0)

	for _, evm_network := range networks {
		client, err := network_client.New(evm_network)
		if err != nil {
			return nil, err
		}

		new_worker := worker.New(client, broadcast_channel, debug)
		go new_worker.Sync()

		workers[client.Network.Id] = new_worker
	}

	return workers, nil
}

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// returns the earliest cached block number
func block_get_cached_number(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}

	if !workers.Exist(network_id) {
		return message.Fail("unsupported network_id " + network_id)
	}

	client := workers.Client(network_id)
	block_number, err := client.GetRecentBlockNumber()
	if err != nil {
		return message.Fail(err.Error())
	}

	block_timestamp, err := client.GetBlockTimestamp(block_number)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    block_number,
			"block_timestamp": block_timestamp,
		}),
	}
}

// Returns the block timestamp
func block_get_timestamp(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_number, err := request.Parameters.GetUint64("block_number")
	if err != nil {
		return message.Fail(err.Error())
	}

	if !workers.Exist(network_id) {
		return message.Fail("unsupported network_id " + network_id)
	}

	client := workers.Client(network_id)
	block_timestamp, err := client.GetBlockTimestamp(block_number)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    block_number,
			"block_timestamp": block_timestamp,
		}),
	}
}

// Returns the transactions and logs in a range of the block.
// Optionally it accepts to parameter that filters the transactions and logs
// for the smartcontract.
func block_get_range(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_numbers, err := request.Parameters.GetUint64s("block_number_from", "block_number_to")
	if err != nil {
		return message.Fail(err.Error())
	}

	to, _ := request.Parameters.GetString("to")

	if !workers.Exist(network_id) {
		return message.Fail("unsupported network_id " + network_id)
	}

	client := workers.Client(network_id)
	earliest_block_number, err := client.GetRecentBlockNumber()
	if err != nil {
		return message.Fail(err.Error())
	}
	if block_numbers[0] < earliest_block_number || block_numbers[1] < earliest_block_number {
		return message.Fail(fmt.Sprintf("please run a worker, the database keeps the blockchain data up until %d", earliest_block_number))
	}

	block_length, err := client.Network.GetFirstProviderLength()
	if err != nil {
		return message.Fail(err.Error())
	}
	recent_block_number := earliest_block_number + block_length
	if block_numbers[0] < recent_block_number || block_numbers[1] < recent_block_number {
		return message.Fail(fmt.Sprintf("please run a worker, the database keeps the blockchain data up until %d", earliest_block_number))
	}

	timestamp, err := client.GetBlockTimestamp(block_numbers[1])

	if err != nil {
		return message.Fail(err.Error())
	}

	var logs []*log.Log = make([]*log.Log, 0)
	var addresses []string = make([]string, 0)
	if to != "" {
		addresses = append(addresses, to)
	}

	raw_logs, err := client.GetBlockRangeLogs(block_numbers[0], block_numbers[1], addresses)
	if err != nil {
		return message.Fail(err.Error())
	}
	logs, err = log.NewLogsFromRaw(network_id, timestamp, raw_logs)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id": network_id,
			"to":         to,
			"timestamp":  timestamp,
			"logs":       data_type.ToMapList(logs),
		}),
	}
}

// this function returns the smartcontract deployer, deployed block number
// and block timestamp by a transaction hash of the smartcontract deployment.
func transaction_deployed_get(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	txid, err := request.Parameters.GetString("txid")
	if err != nil {
		return message.Fail(err.Error())
	}

	if !workers.Exist(network_id) {
		return message.Fail("unsupported network_id " + network_id)
	}

	tx, err := workers.Client(network_id).GetTransaction(txid)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    tx.BlockNumber,
			"block_timestamp": tx.BlockTimestamp,
			"address":         tx.TxTo,
			"deployer":        tx.TxFrom,
			"txid":            txid,
		}),
	}

	return reply
}

// Returns the event logs
// and block timestamp by a transaction hash of the smartcontract deployment.
func log_filter(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_number_from, err := request.Parameters.GetUint64("block_number_from")
	if err != nil {
		return message.Fail(err.Error())
	}

	addresses, err := request.Parameters.GetStringList("addresses")
	if err != nil {
		return message.Fail(err.Error())
	}

	if !workers.Exist(network_id) {
		return message.Fail("unsupported network_id " + network_id)
	}

	length, err := workers.Client(network_id).Network.GetFirstProviderLength()
	if err != nil {
		return message.Fail("failed to get the block range length for first provider of " + network_id)
	}
	block_number_to := block_number_from + length

	raw_logs, err := workers.Client(network_id).GetBlockRangeLogs(block_number_from, block_number_to, addresses)
	if err != nil {
		return message.Fail(err.Error())
	}

	block_timestamp, err := workers.Client(network_id).GetBlockTimestamp(block_number_from)
	if err != nil {
		return message.Fail(err.Error())
	}

	logs, err := log.NewLogsFromRaw(network_id, block_timestamp, raw_logs)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"logs": logs,
		}),
	}

	return reply
}

func Run(app_config *configuration.Config, db_con *db.Database) {
	arguments, err := argument.GetArguments()
	if err != nil {
		panic(err)
	}

	greeting := `SDS Spaghetti preparation...
It supports the following arguments:
    --broadcast-debug   set it to print the spaghetti worker log
    --security-debug    set it to print the security log`
	println(greeting)

	spaghetti_env, err := service.New(service.SPAGHETTI, service.BROADCAST, service.THIS)
	if err != nil {
		panic(err)
	}

	static_env, err := service.New(service.STATIC, service.REMOTE)
	if err != nil {
		panic(err)
	}

	static_socket = remote.TcpRequestSocketOrPanic(static_env, spaghetti_env)
	networks, err := network.GetRemoteNetworks(static_socket, network.WITH_VM)
	if err != nil {
		panic(err)
	}

	// we whitelist before we initiate the reply controller
	if !app_config.Plain {
		whitelisted_services, err := get_whitelisted_services()
		if err != nil {
			panic(err)
		}
		accounts := account.NewServices(whitelisted_services)
		controller.AddWhitelistedAccounts(spaghetti_env, accounts)

		whitelisted_subscribers, err := get_whitelisted_subscribers()
		if err != nil {
			panic(err)
		}
		subsribers := account.NewServices(whitelisted_subscribers)

		broadcast.AddWhitelistedAccounts(spaghetti_env, subsribers)
	}

	reply, err := controller.NewReply(spaghetti_env)
	if err != nil {
		panic(err)
	}

	broadcaster, err := broadcast.New(spaghetti_env)
	if err != nil {
		panic(err)
	}

	if !app_config.Plain {
		err := reply.SetControllerPrivateKey()
		if err != nil {
			panic(err)
		}

		err = broadcaster.SetPrivateKey()
		if err != nil {
			panic(err)
		}
	}

	broadcast_debug := argument.Has(arguments, argument.BROADCAST_DEBUG)
	workers, err = setup_evm_workers(networks, broadcaster.In, broadcast_debug)
	if err != nil {
		panic(err)
	}

	go broadcaster.Run()

	var commands = controller.CommandHandlers{
		"block_get_cached_number":  block_get_cached_number,
		"block_get_timestamp":      block_get_timestamp,
		"block_get_range":          block_get_range,
		"log_filter":               log_filter,
		"transaction_deployed_get": transaction_deployed_get,
	}
	err = reply.Run(db_con, commands)
	if err != nil {
		panic(err)
	}
}
