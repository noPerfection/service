package service

import (
	"fmt"
	"time"

	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// TopologyConnection keeps all necessary parameters of the topology connection service.
type TopologyConnection struct {
	topologyHandler *topology.Handler // topology handles the configuration and dependencies
	topologyClient  *topology.Client
}

func newTopologyConnection() *TopologyConnection {
	topologyConnection := &TopologyConnection{
		topologyHandler: nil,
		topologyClient:  nil,
	}
	return topologyConnection
}

// SetTopologyParams configures the local topology handler before Start.
// Supported keys: "filepath" — topology JSON path (default DefaultConfigPath).
func (topologyConnection *TopologyConnection) SetTopologyParams(params map[string]any) error {
	if topologyConnection.topologyHandler != nil {
		return fmt.Errorf("topology handler already configured")
	}
	if params == nil {
		params = map[string]any{}
	}
	for key := range params {
		if key != TopologyParamFilepath {
			return fmt.Errorf("unsupported topology param %q", key)
		}
	}
	configPath := DefaultConfigPath
	if v, ok := params[TopologyParamFilepath]; ok && v != nil {
		filepath, ok := v.(string)
		if !ok {
			return fmt.Errorf("topology param %q must be string", TopologyParamFilepath)
		}
		if filepath != "" {
			configPath = filepath
		}
	}
	h, err := newTopologyHandler(configPath)
	if err != nil {
		return err
	}
	topologyConnection.topologyHandler = h
	return nil
}

func (topologyConnection *TopologyConnection) topology() topology.TopologyInterface {
	if topologyConnection == nil {
		return nil
	}
	if topologyConnection.topologyClient != nil {
		return topologyConnection.topologyClient
	}
	return topologyConnection.topologyHandler
}

// setupTopologyConnection sets up the topology connection.
func (topologyConnection *TopologyConnection) setupTopologyConnection() error {
	if err := topologyConnection.connectTopologyClientIfRunning(); err != nil {
		return fmt.Errorf("connectTopologyClientIfRunning: %w", err)
	}
	if topologyConnection.topologyClient == nil && topologyConnection.topologyHandler == nil {
		h, err := newTopologyHandler(DefaultConfigPath)
		if err != nil {
			return err
		}
		topologyConnection.topologyHandler = h
	}

	if topologyConnection.topologyHandler != nil {
		if err := topologyConnection.topologyHandler.Start(); err != nil {
			return fmt.Errorf("topologyHandler.Start(): %w", err)
		}
	}
	if err := topologyConnection.ensureTopologyClient(); err != nil {
		return fmt.Errorf("ensureTopologyClient: %w", err)

	}

	return nil
}

func (topologyConnection *TopologyConnection) connectTopologyClientIfRunning() error {
	if topologyConnection == nil || topologyConnection.topologyClient != nil {
		return nil
	}
	probe, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	probe.Timeout(50 * time.Millisecond)
	probe.Attempt(1)
	running, err := probe.IsRunning()
	_ = probe.Close()
	if err != nil || !running {
		return nil
	}
	client, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	topologyConnection.topologyClient = client
	return nil
}

func (topologyConnection *TopologyConnection) ensureTopologyClient() error {
	if topologyConnection == nil || topologyConnection.topologyClient != nil {
		return nil
	}
	client, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	topologyConnection.topologyClient = client
	return nil
}

// depMushroomURL resolves a dep symbol or link to a TopologyURL via topology.GetLink.
func depMushroomURL(tp topology.TopologyInterface, url string) (mushroom.TopologyURL, error) {
	link, err := tp.GetLink(url)
	if err != nil {
		return mushroom.TopologyURL{}, fmt.Errorf("topology.GetLink(%q): %w", url, err)
	}
	u, err := mushroom.New(link)
	if err != nil {
		return mushroom.TopologyURL{}, fmt.Errorf("mushroom.New(%q): %w", link, err)
	}
	return u, nil
}

func (topologyConnection *TopologyConnection) setTopologyHandler(handler config.Handler, url string) error {
	tp := topologyConnection.topology()
	mushroomURL, err := mushroom.New(url)
	if err != nil {
		return fmt.Errorf("mushroom.New(%q): %w", url, err)
	}
	handlerURL := mushroomURL.HandlerLink().AsDereference().String()
	if err := tp.SetHandler(handler, handlerURL); err != nil {
		return fmt.Errorf("topology.SetHandler(%q): %w", handlerURL, err)
	}
	return nil
}

// resolveTopologyHandler resolves a service or handler topology URL to handler config.
// When category=manager is requested but not defined in topology, the default manager
// endpoint for the service type is used.
func (topologyConnection *TopologyConnection) resolveTopologyHandler(url string) (config.Handler, error) {
	mushroomURL, err := mushroom.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("mushroom.Parse(%q): %w", url, err)
	}
	tp := topologyConnection.topology()
	category := mushroomURL.HandlerLink().HandlerCategory()
	serviceURL := mushroomURL.As(mushroom.SERVICE).AsDereference().String()

	if !mushroomURL.IsHandlerExist() {
		service, err := tp.Service(serviceURL)
		if err != nil {
			return nil, fmt.Errorf("topology.Service(%q): %w", mushroomURL, err)
		}
		return service.HandlerByCategory(category)
	}

	handler, err := tp.Handler(mushroomURL.As(mushroom.HANDLER).AsDereference().String())
	if err == nil {
		return handler, nil
	}
	if category != config.ServiceManagerCategory {
		return nil, fmt.Errorf("topology.Handler(%q): %w", mushroomURL, err)
	}

	service, svcErr := tp.Service(serviceURL)
	if svcErr != nil {
		return nil, fmt.Errorf("topology.Service(%q): %w", mushroomURL, svcErr)
	}
	endpoint, epErr := managerEndpointForService(service)
	if epErr != nil {
		return nil, epErr
	}
	return config.IndependentHandler{
		Type:     config.SyncReplierType,
		Category: config.ServiceManagerCategory,
		Endpoint: endpoint,
	}, nil
}

func (topologyConnection *TopologyConnection) closeTopologyClient() error {
	if topologyConnection.topologyClient != nil {
		_ = topologyConnection.topologyClient.Close()
		topologyConnection.topologyClient = nil
	}
	return nil
}
