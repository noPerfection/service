package service

import "github.com/ahmetson/service-lib/configuration"

type Proxy struct {
	Url       string
	Instances []Instance
	Context   configuration.ContextType
}
