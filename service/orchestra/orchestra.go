package orchestra

import (
	"github.com/ahmetson/service-lib/config"
)

type Type = string

const (
	// DevContext indicates that all dependency proxies are in the local machine
	DevContext Type = "development"
	// DefaultContext indicates that all dependencies are in any machine.
	// It's unspecified.
	DefaultContext Type = "default"
)

type Interface interface {
	GetConfig(string) (*config.Service, error) // string arg is the service url
	SetConfig(string, *config.Service) error   // string arg is service url
	Paths() []string
	SetUrl(url string)
	GetUrl() string
	Host() string
	GetType() Type
}
