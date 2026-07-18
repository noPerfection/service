package manager

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const ipcManagerProbeTimeout = 100 * time.Millisecond

// IpcServiceStartAttempts is how many IPC manager probes startIpcServices waits after spawning a dep.
const IpcServiceStartAttempts = 30

func newServiceManagerClient(service config.Service, secretKey, hmacSecret string) (*topology.Client, error) {
	handler, err := service.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return nil, fmt.Errorf("no manager found in the '%s' service, please set its config", service.Name)
	}

	independentHandler, ok := handler.AsIndependentHandler()
	if !ok {
		return nil, fmt.Errorf("manager handler in '%s' is invalid", service.Name)
	}
	socket, err := client.New(independentHandler.Endpoint.Id, independentHandler.Endpoint.Port, client.SyncReplierType)
	if err != nil {
		return nil, fmt.Errorf("client.New: %w", err)
	}
	node := &topology.Client{Socket: socket}
	if service.Parameters != nil {
		if pubKey, ok := service.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
			node.Socket.Allow(pubKey)
		}
	}
	node.Socket.Secure(secretKey)
	if hmacSecret != "" {
		_ = node.Socket.Whitelist(message.Any, hmacSecret)
	}

	return node, nil
}

func managerProbeTimeout(service config.Service) time.Duration {
	managerHandler, err := service.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return topology.DefaultTimeout
	}
	handler, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return topology.DefaultTimeout
	}
	if handler.Endpoint.IsIpc() || handler.Endpoint.IsInproc() {
		return ipcManagerProbeTimeout
	}
	return topology.DefaultTimeout
}

func probeServiceRunning(service config.Service, secretKey, hmacSecret string) (bool, error) {
	if service.Type == config.IndependentType {
		return true, nil
	}

	node, err := newServiceManagerClient(service, secretKey, hmacSecret)
	if err != nil {
		return false, err
	}
	defer node.Close()

	node.Attempt(1)
	node.Timeout(managerProbeTimeout(service))

	running, err := node.Request(&message.Request{
		Command:    topology.IsServiceRunning,
		Parameters: datatype.New().Set("service", service.Name),
	})
	if err != nil {
		if errors.Is(err, message.RequestTimeoutError) {
			return false, nil
		}
		return false, err
	}
	if !running.IsOK() {
		msg := running.ErrorMessage()
		if strings.Contains(msg, message.ErrAccessDenied.Error()) {
			return false, fmt.Errorf("reply.Message: %w (%s)", message.ErrAccessDenied, msg)
		}
		return false, fmt.Errorf("reply.Message: %s", msg)
	}
	isRunning, err := running.ReplyParameters().BoolValue("running")
	if err != nil {
		return false, fmt.Errorf("reply.Parameters.BoolValue('running'): %w", err)
	}
	return isRunning, nil
}

func isServiceRunningWithReload(tp *topology.Client, serviceURL, secretKey, hmacSecret string, attempts ...int) (bool, error) {
	n := 1
	if len(attempts) > 0 && attempts[0] > 1 {
		n = attempts[0]
	}
	reload := n > 1

	for i := 0; i < n; i++ {
		if reload {
			if err := tp.Reload(); err != nil {
				return false, fmt.Errorf("topology.Reload: %w", err)
			}
		}

		service, err := tp.Service(serviceURL)
		if err != nil {
			return false, err
		}

		running, err := probeServiceRunning(service, secretKey, hmacSecret)
		if err != nil {
			if errors.Is(err, message.ErrNoCurveKey) {
				if !reload {
					return false, nil
				} else {
					continue
				}
			}
			return false, err
		}
		if running {
			return true, nil
		}
	}
	return false, nil
}

func stopRemoteService(tp *topology.Client, serviceURL, secretKey, hmacSecret string) error {
	service, err := tp.Service(serviceURL)
	if err != nil {
		return err
	}
	if service.Type == config.IndependentType {
		return nil
	}

	node, err := newServiceManagerClient(service, secretKey, hmacSecret)
	if err != nil {
		return err
	}
	defer node.Close()

	node.Timeout(managerProbeTimeout(service))
	node.Attempt(2)

	return node.StopService(service.Name)
}
