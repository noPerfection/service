package service

import (
	"fmt"
	"sync"

	"github.com/noPerfection/topology/config"
)

// Proxy keeps the minimal proxy service state.
type Proxy struct {
	*WithHardcodedTopology
	name    string
	blocker *sync.WaitGroup
}

// NewProxy returns a new proxy service.
func NewProxy(params ...interface{}) (*Proxy, error) {
	name := DefaultName

	if len(params) > 1 {
		return nil, fmt.Errorf("too many arguments, expected name")
	}
	if len(params) > 0 && params[0] != nil {
		nameArg, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("name argument must be string")
		}
		if len(nameArg) > 0 {
			name = nameArg
		}
	}

	return &Proxy{
		WithHardcodedTopology: NewHardcodedTopologies(name),
		name:                  name,
	}, nil
}

// EnableLogger is a placeholder until proxy handlers and manager are defined.
func (proxy *Proxy) EnableLogger(enable bool) error {
	return nil
}

// Name returns the unique name of the proxy.
func (proxy *Proxy) Name() string {
	return proxy.name
}

// Type returns the configuration type for a proxy service.
func (proxy *Proxy) Type() config.Type {
	return config.ProxyType
}

// Start starts the proxy placeholder.
func (proxy *Proxy) Start() error {
	proxy.blocker = &sync.WaitGroup{}
	proxy.blocker.Add(1)
	return nil
}

func (proxy *Proxy) Stop() error {
	return fmt.Errorf("todo not implemented")
}

func (proxy *Proxy) Wait() {
	if proxy.blocker == nil {
		return
	}
	proxy.blocker.Wait()
}
