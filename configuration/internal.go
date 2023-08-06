package configuration

import (
	"github.com/ahmetson/service-lib/configuration/service"
	"strings"
)

// UrlToFileName converts the given url to the file name. Simply it replaces the slashes with dots.
//
// Url returns the full url to connect to the orchester.
//
// The orchester url is defined from the main service's url.
//
// For example:
//
//	serviceUrl = "github.com/ahmetson/sample-service"
//	contextUrl = "orchester.github.com.ahmetson.sample-service"
//
// This controllerName is set as the server's name in the configuration.
// Then the server package will generate an inproc:// url based on the server name.
func UrlToFileName(url string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url, "/", "."), "\\", ".")
}

func ManagerName(url string) string {
	fileName := UrlToFileName(url)
	return "manager." + fileName
}

func ContextName(url string) string {
	fileName := UrlToFileName(url)
	return "orchester." + fileName
}

func InternalConfiguration(name string) *service.Controller {
	instance := service.Instance{
		Port:               0, // 0 means it's inproc
		Id:                 name + "_instance",
		ControllerCategory: name,
	}

	return &service.Controller{
		Type:      service.SyncReplierType,
		Category:  name,
		Instances: []service.Instance{instance},
	}
}

// ClientUrlParameters return the endpoint to connect to this server from other services
func ClientUrlParameters(name string) (string, uint64) {
	return name, 0
}
