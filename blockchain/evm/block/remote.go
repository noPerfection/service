package block

import (
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type/key_value"

	"github.com/blocklords/gosds/spaghetti/log"
)

func RemoteBlockRange(socket *remote.Socket, networkId string, address string, from uint64, to uint64) (uint64, []*log.Log, error) {
	request := message.Request{
		Command: "block_get_range",
		Parameters: map[string]interface{}{
			"block_number_from": from,
			"block_number_to":   to,
			"to":                address,
			"network_id":        networkId,
		},
	}

	raw_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, nil, err
	}
	parameters := key_value.New(raw_parameters)

	timestamp, err := parameters.GetUint64("timestamp")
	if err != nil {
		return 0, nil, err
	}

	raw_logs, err := parameters.GetKeyValueList("logs")
	if err != nil {
		return 0, nil, err
	}

	logs := make([]*log.Log, len(raw_logs))
	for i, raw := range raw_logs {
		l, err := log.New(raw)
		if err != nil {
			return 0, nil, err
		}
		logs[i] = l
	}

	return timestamp, logs, nil
}
