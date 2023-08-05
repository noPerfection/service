package service

import (
	"github.com/ahmetson/service-lib/configuration/context"
)

const (
	// SourceName of this type should be listed within the controllers in the configuration
	SourceName = "source"

	// DestinationName of this type should be listed within the controllers in the configuration
	DestinationName = "destination"
)

type Proxy struct {
	Url       string
	Instances []Instance
	Context   context.Type
}
