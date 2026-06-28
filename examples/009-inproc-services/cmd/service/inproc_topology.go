package main

import (
	"github.com/noPerfection/service"

	"github.com/noPerfection/service/examples/009-inproc-services/services/entrypoint"
	"github.com/noPerfection/service/examples/009-inproc-services/services/proxy"
)

func startInprocTopology() error {
	inprocTopology, err := service.NewInprocExtension()
	if err != nil {
		return err
	}

	proxy1, err := proxy.New()
	if err != nil {
		return err
	}
	entrypoint1, err := entrypoint.New()
	if err != nil {
		return err
	}
	if err := inprocTopology.SetService(entrypointName, entrypoint1); err != nil {
		return err
	}
	if err := inprocTopology.SetService(defaultProxyName, proxy1); err != nil {
		return err
	}

	return inprocTopology.Start()
}
