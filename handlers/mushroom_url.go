package handlers

import (
	"fmt"

	"github.com/ahmetson/mushroom"
)

// AsHandlerLink returns service's link with the handler category
// as the "category" additional property. For example:
//
//	serviceLink := "pkg:json/.#noPerfection.json?var=services[name:main]"
//
// Returns a handler link:
//
//	pkg:json/.#noPerfection.json?var=services[name:main]&category=main
//
// Why this format? Its used by the noPerfection/topology: `ResolveDep`.
//
// Its also used by the noPerfection/service methods:
//
//	`syncCommandDepProxyOutbounds()`, `handlerDepProxyOutboundTargets()` as the final
//
// outbound target URL for the last proxy chain.
func AsHandlerLink(serviceLink, category string) (string, error) {
	if category == "" {
		return "", fmt.Errorf("handler category is empty")
	}
	if serviceLink == "" {
		return "", fmt.Errorf("service link is empty")
	}

	var soil mushroom.Soil
	hypha, err := soil.Hypha(serviceLink)
	if err != nil {
		return "", fmt.Errorf("soil.Hypha(%q): %w", serviceLink, err)
	}

	linkHypha := hypha.AsLink()
	if linkHypha.AdditionalProps == nil {
		linkHypha.AdditionalProps = map[string]string{}
	}
	linkHypha.AdditionalProps["category"] = category
	return linkHypha.String(), nil
}
