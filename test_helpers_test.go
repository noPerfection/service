package service

import (
	topologyConfig "github.com/noPerfection/topology/config"
)

const rootServicesParent = "pkg:$?*var=services"

func testHandlers(handlers ...topologyConfig.IndependentHandler) []topologyConfig.Handler {
	result := make([]topologyConfig.Handler, len(handlers))
	for i, h := range handlers {
		result[i] = h
	}
	return result
}
