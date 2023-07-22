/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/service"
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
func (independent *Independent) AddController(name string, controller *controller.Controller) {
	independent.controllers.Set(name, controller)
}

func (independent *Independent) RequireProxy(url string) {
	independent.requiredProxies = append(independent.requiredProxies, url)
}

// Pipe the controller to the proxy
func (independent *Independent) Pipe(proxyUrl string, name string) error {
	validProxy := false
	for _, url := range independent.requiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			validProxy = true
			break
		}
	}
	if !validProxy {
		return fmt.Errorf("proxy '%s' url not required. call independent.RequireProxy", proxyUrl)
	}

	if err := independent.controllers.Exist(name); err != nil {
		return fmt.Errorf("controller instance '%s' not added. call independent.AddController: %w", name, err)
	}

	independent.pipelines.Set(proxyUrl, name)

	return nil
}

// returns the extension urls
func (independent *Independent) requiredControllerExtensions() []string {
	var extensions []string
	for _, controllerInterface := range independent.controllers {
		c := controllerInterface.(*controller.Controller)
		extensions = append(extensions, c.RequiredExtensions()...)
	}

	return extensions
}

func (independent *Independent) prepareConfiguration() error {
	// validate the independent itself
	config := independent.configuration
	serviceConfig := independent.configuration.Service
	if len(serviceConfig.Type) == 0 {
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			independent.logger.Fatal("failed to get os context", "error", err)
		}

		serviceConfig = configuration.Service{
			Type:     configuration.IndependentType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != configuration.IndependentType {
		return fmt.Errorf("independent type is overwritten. It's not proxy its '%s'", serviceConfig.Type)
	}

	// validate the controllers
	for name, controllerInterface := range independent.controllers {
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
			port := independent.configuration.GetFreePort()

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

	// todo validate the extensions

	independent.configuration.Service = serviceConfig

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the configuration, it makes sure that dependency is linted.
func (independent *Independent) preparePipelineConfiguration(proxyUrl string, controllerName string) error {
	independent.logger.Info("prepare the pipeline")

	found := false
	for _, requiredProxy := range independent.requiredProxies {
		if strings.Compare(proxyUrl, requiredProxy) == 0 {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proxy '%s' not found. add using independent.RequireProxy()", proxyUrl)
	}

	if err := independent.controllers.Exist(controllerName); err != nil {
		return fmt.Errorf("independent.controllers.Exist of '%s': %w", controllerName, err)
	}

	err := service.PreparePipelineConfiguration(independent.configuration, proxyUrl, controllerName, independent.logger)

	if err != nil {
		return fmt.Errorf("service.PreparePipelineConfiguration: %w", err)
	}
	return nil
}

func (independent *Independent) Prepare() error {
	if len(independent.controllers) == 0 {
		return fmt.Errorf("no controllers. call independent.AddController")
	}

	err := service.PrepareContext(independent.configuration.Context)
	if err != nil {
		return fmt.Errorf("service.PrepareContext: %w", err)
	}

	err = independent.prepareConfiguration()
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	// prepare the configuration and run it
	if len(independent.requiredProxies) > 0 {
		independent.logger.Info("there are some proxies to setup")
		for _, requiredProxy := range independent.requiredProxies {
			if err := service.PrepareProxyConfiguration(requiredProxy, independent.configuration, independent.logger); err != nil {
				return fmt.Errorf("service.PrepareProxyConfiguration of %s: %w", requiredProxy, err)
			}
		}

		if len(independent.pipelines) == 0 {
			return fmt.Errorf("no pipepline to lint the proxy to the controller")
		}

		for requiredProxy, controllerInterface := range independent.pipelines {
			controllerName := controllerInterface.(string)
			if err := independent.preparePipelineConfiguration(requiredProxy, controllerName); err != nil {
				return fmt.Errorf("preparePipelineConfiguration '%s'=>'%s': %w", requiredProxy, controllerName, err)
			}
		}
	}

	requiredExtensions := independent.requiredControllerExtensions()
	if len(requiredExtensions) > 0 {
		independent.logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
	} else {
		independent.logger.Info("no extensions needed")
	}

	for name, controllerInterface := range independent.controllers {
		controller := controllerInterface.(*controller.Controller)

		controllerConfig, err := independent.configuration.Service.GetController(name)
		if err != nil {
			return fmt.Errorf("controller '%s' registered in the independent, no configuration: %w", name, err)
		}

		controller.AddConfig(&controllerConfig)
		requiredExtensions := controller.RequiredExtensions()
		for _, extensionUrl := range requiredExtensions {
			requiredExtension := independent.configuration.Service.GetExtension(extensionUrl)
			controller.AddExtensionConfig(requiredExtension)
		}
	}

	return nil
}

// Run the independent service.
func (independent *Independent) Run() {
	var wg sync.WaitGroup

	for _, controllerConfig := range independent.configuration.Service.Controllers {
		if err := independent.controllers.Exist(controllerConfig.Name); err != nil {
			fmt.Println("the config doesn't exist", controllerConfig, "error", err)
			continue
		}
		controllerList := independent.controllers.Map()
		var c, ok = controllerList[controllerConfig.Name].(*controller.Controller)
		if !ok {
			independent.logger.Fatal("interface -> key-value failed", "controller name")
			continue
		}

		// add the extensions required by the controller
		requiredExtensions := c.RequiredExtensions()
		for _, url := range requiredExtensions {
			extension := independent.configuration.Service.GetExtension(url)
			if extension == nil {
				independent.logger.Fatal("extension required by the controller doesn't exist in the configuration", "url", url)
			}

			c.AddExtensionConfig(extension)
		}

		wg.Add(1)
		go func() {
			err := c.Run()
			wg.Done()
			if err != nil {
				independent.logger.Fatal("failed to run the controller", "error", err)
			}
		}()
	}
	wg.Wait()
}
