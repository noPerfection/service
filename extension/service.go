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

// Service of the extension type
type Service struct {
	configuration configuration.Service
	controllers   []*controller.Controller
}

// New extension service based on the configurations
func New(serviceConf configuration.Service, logger *log.Logger) (*Service, error) {
	if serviceConf.Type != configuration.ExtensionType {
		return nil, fmt.Errorf("service type in the configuration is not Service. It's '%s'", serviceConf.Type)
	}
	service := Service{
		configuration: serviceConf,
		controllers:   make([]*controller.Controller, 0),
	}

	if err := service.initController(logger); err != nil {
		return nil, fmt.Errorf("initController: %w", err)
	}

	return &service, nil
}

// initController takes the first controller from configuration and adds them into the Service.
func (service *Service) initController(logger *log.Logger) error {
	replier, err := controller.NewReplier(logger)
	if err != nil {
		return fmt.Errorf("controller.NewReplier: %w", err)
	}

	controllerConf, err := service.configuration.GetFirstController()
	if err != nil {
		return fmt.Errorf("controller configuration wasn't found: %v", err)
	}
	replier.AddConfig(&controllerConf)

	service.controllers = append(service.controllers, replier)

	return nil
}

// GetFirstController returns the first controller of this extension
func (service *Service) GetFirstController() *controller.Controller {
	return service.controllers[0]
}

// Run the independent service.
func (service *Service) Run() {
	var wg sync.WaitGroup

	c := service.GetFirstController()

	// add the extensions required by the controller
	requiredExtensions := c.RequiredExtensions()
	for _, name := range requiredExtensions {
		extension, err := service.configuration.GetExtension(name)
		if err != nil {
			log.Fatal("extension required by the controller doesn't exist in the configuration", "error", err)
		}

		c.AddExtensionConfig(&extension)
	}

	wg.Add(1)
	go func() {
		err := c.Run()
		wg.Done()
		if err != nil {
			log.Fatal("failed to run the controller", "error", err)
		}
	}()

	wg.Wait()
}
