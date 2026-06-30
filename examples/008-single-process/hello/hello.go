package hello

import (
	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	npservice "github.com/noPerfection/service"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	ServiceName        = "hello-world"
	ServiceManagerPort = 8001
)

func New(configPath string) (*npservice.Independent, error) {
	app, err := npservice.New(ServiceName)
	if err != nil {
		return nil, err
	}
	if configPath != "" {
		if err := app.SetTopologyParams(map[string]any{npservice.TopologyParamFilepath: configPath}); err != nil {
			return nil, err
		}
	}

	if err := app.SetEndpoint(message.NewEndpoint("localhost", ServiceManagerPort), topologyConfig.ServiceManagerCategory); err != nil {
		return nil, err
	}

	if err := app.Route("hello", onHello); err != nil {
		return nil, err
	}
	if err := app.Route("age-verification", onAgeVerification); err != nil {
		return nil, err
	}

	return app, nil
}

func onHello(req message.RequestInterface) message.ReplyInterface {
	name, err := req.RouteParameters().StringValue("name")
	if err != nil || name == "" {
		return req.Fail("name is required")
	}

	return req.Ok(datatype.New().Set("message", "hello "+name))
}

func onAgeVerification(req message.RequestInterface) message.ReplyInterface {
	age, err := req.RouteParameters().Uint64Value("age")
	if err != nil {
		return req.Fail("age is required")
	}

	return req.Ok(datatype.New().Set("passed", age >= 18))
}
