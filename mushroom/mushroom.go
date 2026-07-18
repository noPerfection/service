package mushroom

import (
	"fmt"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/datatype"
	"github.com/noPerfection/topology/config"
)

// The service url or handler url or route url.
//
// For service url it needs to have a resource path.
// For handler url it needs to have a category additional property.
// For route url it needs to have a command additional property.
type TopologyURL mushroom.Hypha

const SERVICE = 1
const HANDLER = 2
const ROUTE = 3
const ManagerPublicKeyParam = "public-key"

// Returns a topology url with the additional properties. Receives two optional parameters to include:
//   - the handler category
//   - the route command
//
// If no optional parameters, category and command embedded in serviceLink are stripped.
// Use Parse to preserve them when re-parsing a handler or route URL.
func New(serviceLink string, params ...string) (TopologyURL, error) {
	var soil mushroom.Soil
	hypha, err := soil.Hypha(serviceLink)
	if err != nil {
		return TopologyURL{}, fmt.Errorf("soil.Hypha(%q): %w", serviceLink, err)
	}
	topologyURL := TopologyURL(hypha)
	if !topologyURL.IsServiceExist() {
		return TopologyURL{}, fmt.Errorf("service link %q is not a service link, please add var resource path", serviceLink)
	}
	if len(params) == 0 {
		delete(hypha.AdditionalProps, "category")
		delete(hypha.AdditionalProps, "command")
		return TopologyURL(hypha), nil
	}
	if len(params) == 1 {
		if hypha.AdditionalProps == nil {
			hypha.AdditionalProps = map[string]string{}
		}
		hypha.AdditionalProps["category"] = params[0]
		delete(hypha.AdditionalProps, "command")
		return TopologyURL(hypha), nil
	}
	if hypha.AdditionalProps == nil {
		hypha.AdditionalProps = map[string]string{}
	}
	hypha.AdditionalProps["category"] = params[0]
	hypha.AdditionalProps["command"] = params[1]
	return TopologyURL(hypha), nil
}

// Parse parses a topology URL and keeps category/command from the link string.
func Parse(serviceLink string) (TopologyURL, error) {
	var soil mushroom.Soil
	hypha, err := soil.Hypha(serviceLink)
	if err != nil {
		return TopologyURL{}, fmt.Errorf("soil.Hypha(%q): %w", serviceLink, err)
	}
	topologyURL := TopologyURL(hypha)
	if !topologyURL.IsServiceExist() {
		return TopologyURL{}, fmt.Errorf("service link %q is not a service link, please add var resource path", serviceLink)
	}
	return topologyURL, nil
}

// Returns a service's link with the handler category
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
func (t TopologyURL) New(category string, command ...string) TopologyURL {
	hypha := TopologyURL(mushroom.Hypha(t).AsLink())
	if hypha.AdditionalProps == nil {
		hypha.AdditionalProps = map[string]string{}
	}
	hypha.AdditionalProps["category"] = category
	if len(command) > 0 {
		hypha.AdditionalProps["command"] = command[0]
	}
	return TopologyURL(hypha)
}

func (t TopologyURL) NewRouteURL(command string) TopologyURL {
	routeURL := t
	if !routeURL.IsHandlerExist() {
		routeURL = t.New(config.DefaultCategory)
	}
	return routeURL.New(routeURL.HandlerCategory(), command)
}

func (t TopologyURL) HandlerCategory() string {
	if t.IsHandlerExist() {
		return t.AdditionalProps["category"]
	}
	return ""
}

// HandlerLink returns a handler topology URL, applying DefaultCategory when category is missing.
func (t TopologyURL) HandlerLink() TopologyURL {
	if t.IsHandlerExist() {
		return t
	}
	return t.New(config.DefaultCategory)
}

func (t TopologyURL) AsDereference() TopologyURL {
	return TopologyURL(mushroom.Hypha(t).AsDereference())
}

func (t TopologyURL) String() string {
	return mushroom.Hypha(t).String()
}

// Its simplified not full assurance. It just needs to make sure it has a variable resource path.
func (t TopologyURL) IsServiceExist() bool {
	return t.ResourcePath.String() != "" && t.ResourceKind == mushroom.ResourceKindVar
}

// It needs to have a service link and a 'category' additional property.
func (t TopologyURL) IsHandlerExist() bool {
	if !t.IsServiceExist() {
		return false
	}
	return t.AdditionalProps != nil && t.AdditionalProps["category"] != ""
}

// It needs to have a handler link and a 'command' additional property.
func (t TopologyURL) IsRouteExist() bool {
	if !t.IsHandlerExist() {
		return false
	}
	return t.AdditionalProps != nil && t.AdditionalProps["command"] != ""
}

// Returns a copy as of 'kind'. For example, given a topology url:
//
//	serviceLink := "pkg:$?var=services[name:main]&category=main&command=any"
//
// serviceLink.As(SERVICE) returns:
//
//	pkg:$?var=services[name:main]
//
// serviceLink.As(HANDLER) returns:
//
//	pkg:$?var=services[name:main]&category=main
//
// serviceLink.As(ROUTE) returns:
//
//	pkg:$?var=services[name:main]&category=main&command=any
func (t TopologyURL) As(kind int) TopologyURL {
	hypha := TopologyURL(mushroom.Hypha(t).AsLink())
	if kind == SERVICE {
		delete(hypha.AdditionalProps, "category")
		delete(hypha.AdditionalProps, "command")
	}
	if kind == HANDLER {
		delete(hypha.AdditionalProps, "command")
	}
	return hypha
}

func (t TopologyURL) Equal(b any, kind int) bool {
	var o TopologyURL
	switch b := b.(type) {
	case TopologyURL:
		o = b
	case string:
		var err error
		o, err = New(b)
		if err != nil {
			return false
		}
	}
	if kind == SERVICE {
		return t.IsServiceExist() && o.IsServiceExist() && mushroom.Hypha(t.As(SERVICE)).Satisfies(mushroom.Hypha(o.As(SERVICE)))
	}
	if kind == HANDLER {
		return t.IsHandlerExist() && o.IsHandlerExist() && mushroom.Hypha(t.As(HANDLER)).Satisfies(mushroom.Hypha(o.As(HANDLER)))
	}
	return t.IsRouteExist() && o.IsRouteExist() && mushroom.Hypha(t.As(ROUTE)).Satisfies(mushroom.Hypha(o.As(ROUTE)))
}

// ResourcePublicKey creates a new copy with the public key parameter as a resource:
//
//	pkg:json/.#noPerfection.json?var=services[name:hello-world].parameters.public-key
func (t TopologyURL) ResourcePublicKey() TopologyURL {
	paramsHypha, _ := mushroom.Hypha(t).AsLink().ChildResource("parameters")
	// ChildResource("public-key") creates a new segment, not a scalar —
	// resulting in …parameters.public-key (map navigation), not …parameters[public-key]
	// (array filter), which would fail because parameters is a map, not an array.
	pubKeyHypha, _ := paramsHypha.ChildResource(ManagerPublicKeyParam)
	return TopologyURL(pubKeyHypha)
}

// Checks is depService's categories handler allows the managerLink to access or not.
// If no categories are given, uses config.ServiceManagerCategory.
func IsAllowedPublicKeyExist(depService *config.Service, managerLink TopologyURL, categories ...string) bool {
	if depService.Parameters == nil {
		return true
	}
	serviceParameters := depService.Parameters
	if serviceParameters == nil {
		return true
	}
	allowed, ok := serviceParameters["allowed"]
	if !ok {
		return true
	}
	categoryMap, ok := allowed.(map[string]any)
	if !ok {
		return true
	}
	category := config.ServiceManagerCategory
	if len(categories) > 0 {
		category = categories[0]
	}
	catEntry, ok := categoryMap[category]
	if !ok {
		return true
	}
	entryMap, ok := catEntry.(map[string]any)
	if !ok {
		return true
	}
	_, exists := entryMap[managerLink.String()]
	return exists
}

// IsAllowedPublicKeyMatch reports whether the manager allow entry is missing or stale.
// Returns false when the stored value already matches.
//
// Looks for the 'allowed' parameter of depService for the categories. If not given uses config.ServiceManagerCategory.
// As a key uses managerLink, as a value uses the dereference of value.
func IsAllowedPublicKeyMatch(depService *config.Service, managerLink, value TopologyURL, categories ...string) bool {
	if depService.Parameters == nil {
		return true
	}
	serviceParameters := depService.Parameters
	if serviceParameters == nil {
		return true
	}
	allowed, ok := serviceParameters["allowed"]
	if !ok {
		return true
	}
	categoryMap, ok := allowed.(map[string]any)
	if !ok {
		return true
	}
	category := config.ServiceManagerCategory
	if len(categories) > 0 {
		category = categories[0]
	}
	catEntry, ok := categoryMap[category]
	if !ok {
		return true
	}
	entryMap, ok := catEntry.(map[string]any)
	if !ok {
		return true
	}
	existing, exists := entryMap[managerLink.String()]
	return !exists || existing != value.AsDereference().String()
}

// AddAllowedPublicKey writes the manager allow entry into depService.Parameters.
// As a key uses managerLink, as a value uses the dereference of value.
// If categories are given, uses the first one. If not given uses config.ServiceManagerCategory.
func AddAllowedPublicKey(depService *config.Service, managerLink, value TopologyURL, categories ...string) {
	if depService.Parameters == nil {
		depService.Parameters = datatype.New()
	}

	var categoryMap map[string]any
	if allowed, ok := depService.Parameters["allowed"]; ok {
		if cm, ok := allowed.(map[string]any); ok {
			categoryMap = cm
		}
	}
	if categoryMap == nil {
		categoryMap = make(map[string]any)
	}

	var entryMap map[string]any
	category := config.ServiceManagerCategory
	if len(categories) > 0 {
		category = categories[0]
	}
	if catEntry, ok := categoryMap[category]; ok {
		if em, ok := catEntry.(map[string]any); ok {
			entryMap = em
		}
	}
	if entryMap == nil {
		entryMap = make(map[string]any)
	}

	entryMap[managerLink.String()] = value.AsDereference().String()
	categoryMap[category] = entryMap
	depService.Parameters["allowed"] = categoryMap
}
