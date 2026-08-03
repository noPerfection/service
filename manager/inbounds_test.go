package manager

import (
	"testing"

	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

type testRouteInboundsHost struct {
	tp           *topology.Client
	serviceURL   mushroom.TopologyURL
	serviceDeref string
}

func (h *testRouteInboundsHost) routeInboundsTopology() *topology.Client {
	return h.tp
}

func (h *testRouteInboundsHost) routeInboundsServiceDeref() (string, error) {
	return h.serviceDeref, nil
}

func (h *testRouteInboundsHost) routeInboundsMushroomURL() (mushroom.TopologyURL, error) {
	return h.serviceURL, nil
}

func (h *testRouteInboundsHost) routeHandlerCommands(category string) ([]string, error) {
	if category == topologyConfig.ServiceManagerCategory {
		return []string{message.Any}, nil
	}
	return nil, nil
}

func TestGetRouteInboundsIncludesInprocManagerExtensionEdges(t *testing.T) {
	helloWorldName := testEndpointID(t, "hello-world")
	aiName := testEndpointID(t, "ai")

	helloWorldService := topologyConfig.Service{
		Type:      topologyConfig.IndependentType,
		Name:      helloWorldName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.ReplierType,
				Category: "main",
				Endpoint: message.NewEndpoint(testEndpointID(t, "hello-world-main"), 8000),
			},
		},
		HandlerDeps: []topologyConfig.DepService{
			{
				Name:       topologyConfig.ServiceManagerCategory,
				Extensions: []string{aiName},
			},
		},
	}
	aiService := topologyConfig.Service{
		Type:      topologyConfig.ExtensionType,
		Name:      aiName,
		ModuleUrl: "github.com/noPerfection/service/manager/test",
		Handlers: []topologyConfig.Handler{
			topologyConfig.IndependentHandler{
				Type:     topologyConfig.SyncReplierType,
				Category: topologyConfig.ServiceManagerCategory,
				Endpoint: message.NewEndpoint(testEndpointID(t, "ai-manager"), 0),
			},
		},
	}

	startTestRuntimeHandler(t, helloWorldService, aiService)

	serviceURL, err := mushroom.Parse("*pkg:$?var=services[name:" + helloWorldName + "]")
	require.NoError(t, err)

	tp, err := topology.NewClient()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tp.Close() })

	host := &testRouteInboundsHost{
		tp:           tp,
		serviceURL:   serviceURL,
		serviceDeref: serviceURL.AsDereference().String(),
	}

	inbounds, err := getRouteInbounds(host)
	require.NoError(t, err)

	aiLink, err := tp.GetLink(aiName)
	require.NoError(t, err)
	aiMushroomURL, err := mushroom.Parse(aiLink)
	require.NoError(t, err)

	aiRoutes, ok := inbounds[aiMushroomURL.As(mushroom.SERVICE).String()]
	require.True(t, ok, "ai service inbounds missing")

	aiAnyRoute := aiMushroomURL.NewRouteURL(message.Any).String()
	aiAnyInbounds, ok := aiRoutes[aiAnyRoute]
	require.True(t, ok, "ai.manager.any route missing")

	managerAnyRoute := serviceURL.New(topologyConfig.ServiceManagerCategory).NewRouteURL(message.Any).String()
	require.Contains(t, aiAnyInbounds, managerAnyRoute)
}
