package network

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// Keeps track of the network types
// That SDS supports.
//
// Refer to the constants
type NetworkType string

const (
	ALL NetworkType = "all" // any blockchain
	EVM NetworkType = "evm" // EVM based blockchains
	IMX NetworkType = "imx" // IMX based blockchains
)

// NewNetworkType from given string
func NewNetworkType(network_type string) (NetworkType, error) {
	new_type := NetworkType(network_type)
	if !new_type.valid() {
		return new_type, fmt.Errorf("unsupported network type %s", network_type)
	}

	return new_type, nil
}

// Whether the given flag is valid Network Flag or not.
func (network_type NetworkType) valid() bool {
	return network_type == ALL || network_type == EVM || network_type == IMX
}

// String format of NetworkType
func (network_type NetworkType) String() string {
	return string(network_type)
}

// NewSockets returns client sockets to the remote network services.
//
// The returned sockets is the key value where key is the network type,
// and value is the socket.
func NewClientSockets(app_config *configuration.Config, logger log.Logger) (key_value.KeyValue, error) {
	evm_service, err := service.NewExternal(service.EVM, service.REMOTE, app_config)
	if err != nil {
		logger.Fatal("service.NewExternal(service.EVM)", "error", err)
	}
	evm_socket, err := remote.NewTcpSocket(evm_service, logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(evm_service)", "message", err)
	}
	imx_service, err := service.NewExternal(service.IMX, service.REMOTE, app_config)
	if err != nil {
		logger.Fatal("service.NewExternal(service.IMX)", "error", err)
	}
	imx_socket, err := remote.NewTcpSocket(imx_service, logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(imx_service)", "message", err)
	}
	network_sockets := key_value.Empty().
		Set(EVM.String(), evm_socket).
		Set(IMX.String(), imx_socket)

	return network_sockets, nil
}
