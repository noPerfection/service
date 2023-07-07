/*Package extension is used to scaffold the extension service
 */
package extension

import (
	"fmt"
	"github.com/Seascape-Foundation/sds-service-lib/configuration"
	"github.com/Seascape-Foundation/sds-service-lib/controller"
	"github.com/Seascape-Foundation/sds-service-lib/log"
	"sync"
)

type Extension struct {
	configuration configuration.Service
	controllers   []*controller.Controller
}

// New Extension service based on the configurations
func New(serviceConf configuration.Service, logger log.Logger) (*Extension, error) {
	if serviceConf.Type != configuration.ExtensionType {
		return nil, fmt.Errorf("service type in the configuration is not Extension. It's '%s'", serviceConf.Type)
	}
	service := Extension{
		configuration: serviceConf,
		controllers:   make([]*controller.Controller, 0),
	}

	if err := service.initController(logger); err != nil {
		return nil, fmt.Errorf("initController: %w", err)
	}

	return &service, nil
}

func (service *Extension) initController(logger log.Logger) error {
	replier, err := controller.NewReplier(logger)
	if err != nil {
		return fmt.Errorf("controller.NewReplier: %w", err)
	}

	controllerConf, err := service.configuration.GetFirstController()
	if err != nil {
		return fmt.Errorf("controller configuration wasn't found: %v", err)
	}
	replier.AddConfig(controllerConf)

	service.controllers = append(service.controllers, replier)

	return nil
}

// GetFirstController returns the first controller of this extension
func (service *Extension) GetFirstController() *controller.Controller {
	return service.controllers[0]
}

// Run the independent service.
func (service *Extension) Run() {
	var wg sync.WaitGroup

	for _, c := range service.controllers {
		// add the extensions required by the controller
		requiredExtensions := c.RequiredExtensions()
		for _, name := range requiredExtensions {
			extension, err := service.configuration.GetExtension(name)
			if err != nil {
				log.Fatal("extension required by the controller doesn't exist in the configuration", "error", err)
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

	wg.Wait()
}
