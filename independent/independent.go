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
	"github.com/ahmetson/service-lib/proxy"
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

// if the proxy was given in the configuration, make sure that the file exists there
func (independent *Independent) prepareProxyConfiguration(requiredProxy string) error {
	independent.logger.Info("preparing the proxy", "url", requiredProxy)

	context := independent.configuration.Context

	err := dev.PrepareProxyConfiguration(context, requiredProxy, independent.logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareProxyConfiguration on %s: %w", requiredProxy, err)
	}

	proxy, err := dev.ReadProxyConfiguration(context, requiredProxy)
	if err != nil {
		return fmt.Errorf("dev.ReadProxyConfiguration: %w", err)
	}

	proxyConfiguration := independent.configuration.Service.GetProxy(requiredProxy)
	if proxyConfiguration == nil {
		independent.configuration.Service.SetProxy(proxy)
	} else {
		if strings.Compare(proxyConfiguration.Url, proxy.Url) != 0 {
			return fmt.Errorf("the proxy urls are not matching. in your configuration: %s, in the deps: %s", proxyConfiguration.Url, proxy.Url)
		}
		if proxyConfiguration.Port != proxy.Port {
			return fmt.Errorf("the proxy ports are not matching. in your configuration: %d, in the deps: %d", proxyConfiguration.Port, proxy.Port)
		}
	}

	return nil
}

// preparePipeline checks that proxy url and controllerName are valid.
// Then, in the configuration, it makes sure that dependency is linted.
func (independent *Independent) preparePipeline(proxyUrl string, controllerName string) error {
	independent.logger.Info("prepare the pipeline")

	//
	// make sure that proxy url is valid
	//------------------------------------------------
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

	//
	// make sure that controller name is valid
	//---------------------------------------------------
	if err := independent.controllers.Exist(controllerName); err != nil {
		return fmt.Errorf("independent.controllers.Exist of '%s': %w", controllerName, err)
	}

	//
	// lint the dependency proxy's destination to the independent independent's controller
	//--------------------------------------------------
	context := independent.configuration.Context

	proxyConfig, err := dev.ReadServiceConfiguration(context, proxyUrl)
	if err != nil {
		return fmt.Errorf("dev.ReadServiceConfiguration of '%s': %w", proxyUrl, err)
	}

	destinationConfig, err := proxyConfig.GetController(proxy.DestinationName)
	if err != nil {
		return fmt.Errorf("getting dependency proxy's destination configuration failed: %w", err)
	}

	controllerConfig, err := independent.configuration.Service.GetController(controllerName)
	if err != nil {
		return fmt.Errorf("getting '%s' controller from independent configuration failed: %w", controllerName, err)
	}

	// somehow it will work with only one instance. but in the future maybe another instances as well.
	destinationInstanceConfig := controllerConfig.Instances[0]
	instanceConfig := destinationConfig.Instances[0]

	if destinationInstanceConfig.Port != instanceConfig.Port {
		independent.logger.Info("the dependency proxy destination not match to the controller",
			"proxy url", proxyUrl,
			"destination port", destinationInstanceConfig.Port,
			"independent controller port", instanceConfig.Port)

		destinationInstanceConfig.Port = instanceConfig.Port
		destinationConfig.Instances[0] = destinationInstanceConfig
		proxyConfig.SetController(destinationConfig)

		independent.logger.Info("linting dependency proxy's destination port", "new port", instanceConfig.Port)
		independent.logger.Warn("todo", 1, "if dependency proxy is running, then it should be restarted")
		err := dev.WriteServiceConfiguration(context, proxyUrl, proxyConfig)
		if err != nil {
			return fmt.Errorf("dev.WriteServiceConfiguration for '%s': %w", proxyUrl, err)
		}
	}

	return nil
}

func (independent *Independent) Prepare() error {
	if len(independent.controllers) == 0 {
		return fmt.Errorf("no controllers. call independent.AddController")
	}

	// get the extensions
	err := dev.Prepare(independent.configuration.Context)
	if err != nil {
		return fmt.Errorf("failed to prepare the context: %w", err)
	}

	err = independent.prepareConfiguration()
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	// prepare the configuration and run it
	if len(independent.requiredProxies) > 0 {
		independent.logger.Info("there are some proxies to setup")
		for _, requiredProxy := range independent.requiredProxies {
			if err := independent.prepareProxyConfiguration(requiredProxy); err != nil {
				return fmt.Errorf("prepareProxyConfiguration of %s: %w", requiredProxy, err)
			}
		}

		if len(independent.pipelines) == 0 {
			return fmt.Errorf("no pipepline to lint the proxy to the controller")
		}

		for requiredProxy, controllerInterface := range independent.pipelines {
			controllerName := controllerInterface.(string)
			if err := independent.preparePipeline(requiredProxy, controllerName); err != nil {
				return fmt.Errorf("preparePipeline '%s'=>'%s': %w", requiredProxy, controllerName, err)
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
