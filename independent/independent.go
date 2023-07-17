/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"sync"
)

type Independent struct {
	configuration configuration.Service
	controllers   key_value.KeyValue
}

// New Independent service based on the configurations
func New(serviceConf configuration.Service) (*Independent, error) {
	if serviceConf.Type != configuration.IndependentType {
		return nil, fmt.Errorf("service type in the configuration is not Independent. It's '%s'", serviceConf.Type)
	}
	independent := Independent{
		configuration: serviceConf,
		controllers:   key_value.Empty(),
	}

	return &independent, nil
}

func (service *Independent) AddController(name string, controller *controller.Controller) error {
	controllerConf, err := service.configuration.GetController(name)
	if err != nil {
		return fmt.Errorf("the '%s' controller configuration wasn't found: %v", name, err)
	}
	controller.AddConfig(controllerConf)
	service.controllers.Set(name, controller)

	return nil
}

// Run the independent service.
func (service *Independent) Run() {
	var wg sync.WaitGroup

	for _, c := range service.configuration.Controllers {
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
	println("waiting for the wait group")
	wg.Wait()
}
