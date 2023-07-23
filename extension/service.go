/*Package extension is used to scaffold the extension service
 */
package extension

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"sync"
)

// Extension of the extension type
type Extension struct {
	configuration *configuration.Config
	Controller    *controller.Controller
	logger        *log.Logger
}

// New extension service based on the configurations
func New(config *configuration.Config, logger *log.Logger) (*Extension, error) {
	service := Extension{
		configuration: config,
		Controller:    nil,
		logger:        logger,
	}

	return &service, nil
}

// AddController creates a controller of this extension
func (service *Extension) AddController(controllerType configuration.Type) error {
	if controllerType == configuration.UnknownType {
		return fmt.Errorf("unknown controller type can't be in the extension")
	}

	controllerLogger := service.logger.Child("controller")

	if controllerType == configuration.ReplierType {
		replier, err := controller.NewReplier(controllerLogger)
		if err != nil {
			return fmt.Errorf("controller.NewReplier: %w", err)
		}
		service.Controller = replier
	} else if controllerType == configuration.RouterType {
		//router, err := controller.NewRouter(controllerLogger)
		//if err != nil {
		//	return fmt.Errorf("controller.NewRouter: %w", err)
		//}
		//service.Controller = router
	} else if controllerType == configuration.PusherType {
		puller, err := controller.NewPull(controllerLogger)
		if err != nil {
			return fmt.Errorf("controller.NewPuller: %w", err)
		}
		service.Controller = puller
	}

	return nil
	// code snippet below should be added to the Prepare function
	//controllerConf, err := service.configuration.GetFirstController()
	//if err != nil {
	//	return fmt.Errorf("controller configuration wasn't found: %v", err)
	//}
	//replier.AddConfig(&controllerConf)
}

func (service *Extension) prepareConfiguration() error {
	// validate the service itself
	config := service.configuration
	serviceConfig := service.configuration.Service
	if len(serviceConfig.Type) == 0 {
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			service.logger.Fatal("failed to get os context", "error", err)
		}

		serviceConfig = configuration.Service{
			Type:     configuration.ExtensionType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != configuration.ExtensionType {
		return fmt.Errorf("service type is overwritten. It's not extension. It's '%s'", serviceConfig.Type)
	}

	// validate the controllers
	// it means it should have two controllers: source and destination
	var controllerConfig configuration.Controller
	if len(serviceConfig.Controllers) > 1 {
		return fmt.Errorf("supports one controller only")
	} else if len(serviceConfig.Controllers) == 1 {
		controllerConfig = configuration.Controller{
			Type: service.Controller.ControllerType(),
			Name: config.Name + "controller",
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, controllerConfig)
	} else {
		if controllerConfig.Type != service.Controller.ControllerType() {
			return fmt.Errorf("controller is expected to be of %s type, but in the config it's %s of type",
				service.Controller.ControllerType(), controllerConfig.Type)
		}
	}

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
	} else {
		if controllerConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	// let's validate the extensions
	// it needs to find the extensions using hub or by url
	//
	// then it needs to install the extensions under the ./bin/
	// then it needs to generate the configurations
	// then it needs to load up the generation.
	// then it needs to get the loaded application.
	//
	// if extensions are not provide, then user can set a custom path to the extension
	// using hub
	// instead of the names, use the url

	service.configuration.Service = serviceConfig

	return nil
}

// Prepare the service by validating the configuration.
// if the configuration doesn't exist, it will be created.
func (service *Extension) Prepare() error {
	if service.Controller == nil {
		return fmt.Errorf("missing controller. call AddController")
	}

	if err := service.prepareConfiguration(); err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	// register configuration of the controller
	service.Controller.AddConfig(&service.configuration.Service.Controllers[0])

	// add the extensions required by the controller
	requiredExtensions := service.Controller.RequiredExtensions()
	for _, url := range requiredExtensions {
		extension := service.configuration.Service.GetExtension(url)
		if extension == nil {
			log.Fatal("extension required by the controller doesn't exist in the configuration", "url", url)
		}

		service.Controller.AddExtensionConfig(extension)
	}

	return nil
}

// Run the independent service.
func (service *Extension) Run() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		err := service.Controller.Run()
		wg.Done()
		if err != nil {
			log.Fatal("failed to run the controller", "error", err)
		}
	}()

	wg.Wait()
}
