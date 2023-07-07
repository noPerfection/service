/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/Seascape-Foundation/sds-common-lib/data_type/key_value"
	"github.com/Seascape-Foundation/sds-service-lib/configuration"
	"github.com/Seascape-Foundation/sds-service-lib/controller"
	"github.com/Seascape-Foundation/sds-service-lib/log"
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
	for _, c := range service.configuration.Controllers {
		if err := service.controllers.Exist(c.Name); err != nil {
			continue
		}
		kv, err := service.controllers.GetKeyValue(c.Name)
		if err != nil {
			continue
		}
		var c *controller.Controller
		err = kv.ToInterface(c)
		if err != nil {
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

		go func() {
			err := c.Run()
			if err != nil {
				log.Fatal("failed to run the controller", "error", err)
			}
		}()
	}
}
