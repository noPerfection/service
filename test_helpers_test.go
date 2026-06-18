package service

import (
	"fmt"

	topologyConfig "github.com/noPerfection/topology/config"
)

const rootServicesParent = "*pkg:$?var=services"

func linkTarget(serviceName string, handlerCategory ...string) topologyConfig.DepTarget {
	link := fmt.Sprintf("pkg:$?var=services[name:%s]", serviceName)
	if len(handlerCategory) > 0 && handlerCategory[0] != "" {
		link = fmt.Sprintf("pkg:$?var=services[name:%s].handlers[category:%s]", serviceName, handlerCategory[0])
	}
	return topologyConfig.NewLinkTarget(link)
}

func testHandlers(handlers ...topologyConfig.IndependentHandler) []topologyConfig.Handler {
	result := make([]topologyConfig.Handler, len(handlers))
	for i, h := range handlers {
		result[i] = h
	}
	return result
}
