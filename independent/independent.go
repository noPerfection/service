/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/context/dev"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"strings"
	"sync"
)

// Independent service is the collection of the various controllers
type Independent struct {
	configuration   *configuration.Config
	controllers     key_value.KeyValue
	pipelines       key_value.KeyValue
	requiredProxies []string
	logger          *log.Logger
}

// New Independent service based on the configurations
func New(config *configuration.Config, logger *log.Logger) (*Independent, error) {
	//if serviceConf.Type != configuration.IndependentType {
	//	return nil, fmt.Errorf("service type in the configuration is not Independent. It's '%s'", serviceConf.Type)
	//}
	independent := Independent{
		configuration:   config,
		logger:          logger,
		controllers:     key_value.Empty(),
		requiredProxies: []string{},
		pipelines:       key_value.Empty(),
	}

	return &independent, nil
}

// AddController by their instance name
func (service *Independent) AddController(name string, controller *controller.Controller) {
	//controllerConf, err := service.configuration.GetController(name)
	//if err != nil {
	//	return fmt.Errorf("the '%s' controller configuration wasn't found: %v", name, err)
	//}
	//controller.AddConfig(&controllerConf)
	service.controllers.Set(name, controller)
}

func (service *Independent) RequireProxy(url string) {
	service.requiredProxies = append(service.requiredProxies, url)
}

// Pipe the controller to the proxy
func (service *Independent) Pipe(proxyUrl string, name string) error {
	validProxy := false
	for _, url := range service.requiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			validProxy = true
			break
		}
	}
	if !validProxy {
		return fmt.Errorf("proxy '%s' url not required. call service.RequireProxy", proxyUrl)
	}

	if err := service.controllers.Exist(name); err != nil {
		return fmt.Errorf("controller instance '%s' not added. call service.AddController: %w", name, err)
	}

	service.pipelines.Set(proxyUrl, name)

	return nil
}

// returns the extension urls
func (service *Independent) getExtensions() []string {
	var extensions []string
	for _, controllerInterface := range service.controllers {
		c := controllerInterface.(*controller.Controller)
		extensions = append(extensions, c.RequiredExtensions()...)
	}

	return extensions
}

func (service *Independent) prepareConfiguration() error {
	// validate the service itself
	config := service.configuration
	serviceConfig := service.configuration.Service
	if len(serviceConfig.Type) == 0 {
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			service.logger.Fatal("failed to get os context", "error", err)
		}

		serviceConfig = configuration.Service{
			Type:     configuration.IndependentType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != configuration.IndependentType {
		return fmt.Errorf("service type is overwritten. It's not proxy its '%s'", serviceConfig.Type)
	}

	// validate the controllers
	for name, controllerInterface := range service.controllers {
		c := controllerInterface.(*controller.Controller)

		found := false
		for _, controllerConfig := range serviceConfig.Controllers {
			if controllerConfig.Name == name {
				found = true
				if controllerConfig.Type != c.ControllerType() {
					return fmt.Errorf("controller is expected to be of %s type, but in the config it's %s of type",
						c.ControllerType(), controllerConfig.Type)
				}
			}
		}
		if !found {
			controllerConfig := configuration.Controller{
				Type: c.ControllerType(),
				Name: name,
			}

			serviceConfig.Controllers = append(serviceConfig.Controllers, controllerConfig)
		}
	}

	// validate the controller instances
	for i, controllerConfig := range serviceConfig.Controllers {
		// validate the controller instances
		// make sure that they are tpc type
		if len(controllerConfig.Instances) == 0 {
			port := service.configuration.GetFreePort()

			sourceInstance := configuration.ControllerInstance{
				Name:     controllerConfig.Name,
				Instance: controllerConfig.Name + "1",
				Port:     uint64(port),
			}
			controllerConfig.Instances = append(controllerConfig.Instances, sourceInstance)
			serviceConfig.Controllers[i] = controllerConfig
		} else {
			if controllerConfig.Instances[0].Port == 0 {
				return fmt.Errorf("the port should not be 0 in the source")
			}
		}
	}

	// validate the extensions
	// validate the proxies

	service.configuration.Service = serviceConfig

	return nil
}

// if the proxy was given in the configuration, make sure that the file exists there
func (service *Independent) prepareProxyConfiguration(requiredProxy string) error {
	service.logger.Info("preparing the proxy", "url", requiredProxy)

	context := service.configuration.Context
	proxyConfiguration := service.configuration.Service.GetProxy(requiredProxy)
	if proxyConfiguration == nil {
		err := dev.PrepareProxyConfiguration(context, requiredProxy, service.logger)
		if err != nil {
			return fmt.Errorf("failed to check existence of %s: %w", requiredProxy, err)
		} else {
			service.logger.Warn("dev.PrepareProxyConfiguration should return the proxy to add it to the configuration")
		}
	}

	return nil
}

func (service *Independent) Prepare() error {
	if len(service.controllers) == 0 {
		return fmt.Errorf("no controllers. call service.AddController")
	}

	// get the extensions
	err := dev.Prepare(service.configuration.Context)
	if err != nil {
		return fmt.Errorf("failed to prepare the context: %w", err)
	}

	err = service.prepareConfiguration()
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	// prepare the configuration and run it
	if len(service.requiredProxies) > 0 {
		service.logger.Info("there are some proxies to setup")
		for _, requiredProxy := range service.requiredProxies {
			if err := service.prepareProxyConfiguration(requiredProxy); err != nil {
				return fmt.Errorf("prepareProxyConfiguration of %s: %w", requiredProxy, err)
			}
		}
	}

	controllers := service.controllers.Map()
	for _, controllerConfig := range service.configuration.Service.Controllers {
		controller := controllers[controllerConfig.Name].(*controller.Controller)

		controller.AddConfig(&controllerConfig)
	}

	return nil
}

// Run the independent service.
func (service *Independent) Run() {
	var wg sync.WaitGroup

	for _, c := range service.configuration.Service.Controllers {
		if err := service.controllers.Exist(c.Name); err != nil {
			fmt.Println("the config doesn't exist", c, "error", err)
			continue
		}
		controllerList := service.controllers.Map()
		var c, ok = controllerList[c.Name].(*controller.Controller)
		if !ok {
			fmt.Println("interface -> key-value", c)
			continue
		}

		// add the extensions required by the controller
		requiredExtensions := c.RequiredExtensions()
		for _, url := range requiredExtensions {
			extension := service.configuration.Service.GetExtension(url)
			if extension == nil {
				log.Fatal("extension required by the controller doesn't exist in the configuration", "url", url)
			}

			c.AddExtensionConfig(extension)
		}

		wg.Add(1)
		go func() {
			err := c.Run()
			wg.Done()
			if err != nil {
				log.Fatal("failed to run the controller", "error", err)
			}
		}()
	}
	println("waiting for the wait group")
	wg.Wait()
}
