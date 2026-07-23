package service

import (
	"errors"
	"fmt"

	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

type ipcLifecycle struct {
	topology topology.TopologyInterface
	manager  *manager.Manager
	started  map[string]struct{}
}

func (lifecycle *ipcLifecycle) stopOwnedIpcServices(owner config.Service) error {
	if lifecycle == nil || lifecycle.topology == nil || lifecycle.manager == nil {
		return nil
	}
	stoppedRefs := make(map[string]struct{})
	return lifecycle.stopIpcDepsFor(owner, stoppedRefs)
}

func (lifecycle *ipcLifecycle) stopIpcDepsFor(serviceConfig config.Service, stoppedRefs map[string]struct{}) error {
	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			if err := lifecycle.stopIpcService(proxy, stoppedRefs); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Proxies[%q]: %w", dep.Name, proxy, err)
			}
		}
		for _, extension := range dep.Extensions {
			if err := lifecycle.stopIpcService(extension, stoppedRefs); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Extensions[%q]: %w", dep.Name, extension, err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, proxy := range dep.Proxies {
				if err := lifecycle.stopIpcService(proxy, stoppedRefs); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Proxies[%q]: %w", handler.Category, dep.Name, proxy, err)
				}
			}
			for _, extension := range dep.Extensions {
				if err := lifecycle.stopIpcService(extension, stoppedRefs); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Extensions[%q]: %w", handler.Category, dep.Name, extension, err)
				}
			}
		}
	}

	return nil
}

func (lifecycle *ipcLifecycle) stopIpcService(url string, stoppedRefs map[string]struct{}) error {
	if url == "" {
		return fmt.Errorf("dep mushroom url is empty")
	}

	link, err := lifecycle.topology.GetLink(url)
	if err != nil {
		return fmt.Errorf("topology.GetLink(%q): %w", url, err)
	}
	mushroomURL, err := mushroom.Parse(link)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", link, err)
	}

	depService, err := lifecycle.topology.Service(mushroomURL.AsDereference().String())
	if err != nil {
		return err
	}
	if _, done := stoppedRefs[depService.Name]; done {
		return nil
	}
	stoppedRefs[depService.Name] = struct{}{}

	if depService.IsIpc() && lifecycle.started != nil {
		if _, started := lifecycle.started[depService.Name]; started {
			if err := lifecycle.manager.StopService(depService.Name); err != nil {
				return fmt.Errorf("manager.StopService(%q): %w", depService.Name, err)
			}
		}
	}

	return lifecycle.stopIpcDepsFor(depService, stoppedRefs)
}

func markIpcStarted(started map[string]struct{}, name string) map[string]struct{} {
	if started == nil {
		started = make(map[string]struct{})
	}
	started[name] = struct{}{}
	return started
}

func joinStopErrors(values ...error) error {
	var stopErr error
	for _, err := range values {
		if err != nil {
			stopErr = errors.Join(stopErr, err)
		}
	}
	return stopErr
}
