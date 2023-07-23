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

// Service is the collection of the various Controllers
type Service struct {
	Config          *configuration.Config
	Controllers     key_value.KeyValue
	Pipelines       key_value.KeyValue
	RequiredProxies []string
	Logger          *log.Logger
}

// New service based on the configurations
func New(config *configuration.Config, logger *log.Logger) (*Service, error) {
	//if serviceConf.Type != Config.IndependentType {
	//	return nil, fmt.Errorf("service type in the Config is not Service. It's '%s'", serviceConf.Type)
	//}
	independent := Service{
		Config:          config,
		Logger:          logger,
		Controllers:     key_value.Empty(),
		RequiredProxies: []string{},
		Pipelines:       key_value.Empty(),
	}

	return &independent, nil
}

// AddController by their instance name
func (independent *Service) AddController(name string, controller controller.Interface) {
	independent.Controllers.Set(name, controller)
}

func (independent *Service) RequireProxy(url string) {
	independent.RequiredProxies = append(independent.RequiredProxies, url)
}

// Pipe the controller to the proxy
func (independent *Service) Pipe(proxyUrl string, name string) error {
	validProxy := false
	for _, url := range independent.RequiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			validProxy = true
			break
		}
	}
	if !validProxy {
		return fmt.Errorf("proxy '%s' url not required. call independent.RequireProxy", proxyUrl)
	}

	if err := independent.Controllers.Exist(name); err != nil {
		return fmt.Errorf("controller instance '%s' not added. call independent.AddController: %w", name, err)
	}

	independent.Pipelines.Set(proxyUrl, name)

	return nil
}

// returns the extension urls
func (independent *Service) requiredControllerExtensions() []string {
	var extensions []string
	for _, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		extensions = append(extensions, c.RequiredExtensions()...)
	}

	return extensions
}

func (independent *Service) prepareConfiguration() error {
	// validate the independent itself
	config := independent.Config
	serviceConfig := independent.Config.Service
	if len(serviceConfig.Type) == 0 {
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			independent.Logger.Fatal("failed to get os context", "error", err)
		}

		serviceConfig = configuration.Service{
			Type:     configuration.IndependentType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != configuration.IndependentType {
		return fmt.Errorf("independent type is overwritten. It's not proxy its '%s'", serviceConfig.Type)
	}

	// validate the Controllers
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)

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
			port := independent.Config.GetFreePort()

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

	independent.Config.Service = serviceConfig

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the Config, it makes sure that dependency is linted.
func (independent *Service) preparePipelineConfiguration(proxyUrl string, controllerName string) error {
	independent.Logger.Info("prepare the pipeline")

	found := false
	for _, requiredProxy := range independent.RequiredProxies {
		if strings.Compare(proxyUrl, requiredProxy) == 0 {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proxy '%s' not found. add using independent.RequireProxy()", proxyUrl)
	}

	if err := independent.Controllers.Exist(controllerName); err != nil {
		return fmt.Errorf("independent.Controllers.Exist of '%s': %w", controllerName, err)
	}

	err := service.PreparePipelineConfiguration(independent.Config, proxyUrl, controllerName, independent.Logger)

	if err != nil {
		return fmt.Errorf("service.PreparePipelineConfiguration: %w", err)
	}
	return nil
}

func (independent *Service) Prepare() error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call independent.AddController")
	}

	err := service.PrepareContext(independent.Config.Context)
	if err != nil {
		return fmt.Errorf("service.PrepareContext: %w", err)
	}

	err = independent.prepareConfiguration()
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	// prepare the Config and run it
	if len(independent.RequiredProxies) > 0 {
		independent.Logger.Info("there are some proxies to setup")
		for _, requiredProxy := range independent.RequiredProxies {
			if err := service.PrepareProxyConfiguration(requiredProxy, independent.Config, independent.Logger); err != nil {
				return fmt.Errorf("service.PrepareProxyConfiguration of %s: %w", requiredProxy, err)
			}
		}

		if len(independent.Pipelines) == 0 {
			return fmt.Errorf("no pipepline to lint the proxy to the controller")
		}

		for requiredProxy, controllerInterface := range independent.Pipelines {
			controllerName := controllerInterface.(string)
			if err := independent.preparePipelineConfiguration(requiredProxy, controllerName); err != nil {
				return fmt.Errorf("preparePipelineConfiguration '%s'=>'%s': %w", requiredProxy, controllerName, err)
			}
		}
	}

	requiredExtensions := independent.requiredControllerExtensions()
	if len(requiredExtensions) > 0 {
		independent.Logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
	} else {
		independent.Logger.Info("no extensions needed")
	}

	for name, controllerInterface := range independent.Controllers {
		controller := controllerInterface.(controller.Interface)

		controllerConfig, err := independent.Config.Service.GetController(name)
		if err != nil {
			return fmt.Errorf("controller '%s' registered in the independent, no Config: %w", name, err)
		}

		controller.AddConfig(&controllerConfig)
		requiredExtensions := controller.RequiredExtensions()
		for _, extensionUrl := range requiredExtensions {
			requiredExtension := independent.Config.Service.GetExtension(extensionUrl)
			controller.AddExtensionConfig(requiredExtension)
		}
	}

	return nil
}

// Run the independent service.
func (independent *Service) Run() {
	var wg sync.WaitGroup

	for _, controllerConfig := range independent.Config.Service.Controllers {
		if err := independent.Controllers.Exist(controllerConfig.Name); err != nil {
			fmt.Println("the config doesn't exist", controllerConfig, "error", err)
			continue
		}
		controllerList := independent.Controllers.Map()
		var c, ok = controllerList[controllerConfig.Name].(controller.Interface)
		if !ok {
			independent.Logger.Fatal("interface -> key-value failed", "controller name")
			continue
		}

		// add the extensions required by the controller
		requiredExtensions := c.RequiredExtensions()
		for _, url := range requiredExtensions {
			extension := independent.Config.Service.GetExtension(url)
			if extension == nil {
				independent.Logger.Fatal("extension required by the controller doesn't exist in the Config", "url", url)
			}

			c.AddExtensionConfig(extension)
		}

		wg.Add(1)
		go func() {
			err := c.Run()
			wg.Done()
			if err != nil {
				independent.Logger.Fatal("failed to run the controller", "error", err)
			}
		}()
	}
	wg.Wait()
}
