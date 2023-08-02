package configuration

import (
	"strings"
)

// UrlToFileName converts the given url to the file name. Simply it replaces the slashes with dots.
//
// Url returns the full url to connect to the context.
//
// The context url is defined from the main service's url.
//
// For example:
//
//	serviceUrl = "github.com/ahmetson/sample-service"
//	contextUrl = "context.github.com.ahmetson.sample-service"
//
// This controllerName is set as the controller's name in the configuration.
// Then the controller package will generate an inproc:// url based on the controller name.
func UrlToFileName(url string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url, "/", "."), "\\", ".")
}

func ManagerName(url string) string {
	fileName := UrlToFileName(url)
	return "manager." + fileName
}

func ContextName(url string) string {
	fileName := UrlToFileName(url)
	return "context." + fileName
}

func InternalConfiguration(name string) *Controller {
	instance := ControllerInstance{
		Port:     0, // 0 means it's inproc
		Instance: name + "_instance",
		Name:     name,
	}

	return &Controller{
		Type:      SyncReplierType,
		Name:      name,
		Instances: []ControllerInstance{instance},
	}
}

// ClientUrlParameters return the endpoint to connect to this controller from other services
func ClientUrlParameters(name string) (string, uint64) {
	return name, 0
}
