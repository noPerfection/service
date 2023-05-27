package network

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/configuration"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/parameter"
	"github.com/blocklords/sds/service/remote"
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

// ServiceType returns registered service type
//
// If the network type is not registered, then it's casted into
// custom service type.
func (network_type NetworkType) ServiceType() parameter.ServiceType {
	switch network_type {
	case EVM:
		return parameter.EVM
	case IMX:
		return parameter.IMX
	}

	return parameter.ServiceType(network_type)
}

// NewSockets returns client sockets to the remote network services.
//
// The returned sockets is the key value where key is the network type,
// and value is the socket.
func NewClientSockets(app_config *configuration.Config, logger log.Logger) (key_value.KeyValue, error) {
	evm_service, err := parameter.NewExternal(parameter.EVM, parameter.REMOTE, app_config)
	if err != nil {
		logger.Fatal("parameter.NewExternal(parameter.EVM)", "error", err)
	}
	evm_socket, err := remote.NewTcpSocket(evm_service, &logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(evm_service)", "message", err)
	}
	imx_service, err := parameter.NewExternal(parameter.IMX, parameter.REMOTE, app_config)
	if err != nil {
		logger.Fatal("parameter.NewExternal(parameter.IMX)", "error", err)
	}
	imx_socket, err := remote.NewTcpSocket(imx_service, &logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(imx_service)", "message", err)
	}
	network_sockets := key_value.Empty().
		Set(EVM.String(), evm_socket).
		Set(IMX.String(), imx_socket)

	return network_sockets, nil
}
