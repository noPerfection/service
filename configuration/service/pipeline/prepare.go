package pipeline

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
)

// PrepareAddingPipeline is used to validate the parameters
func PrepareAddingPipeline(pipelines []Pipeline, proxies key_value.KeyValue, controllers key_value.KeyValue, pipeline *Pipeline) error {
	if !pipeline.HasLength() {
		return fmt.Errorf("no proxy")
	}
	if err := pipeline.ValidateHead(); err != nil {
		return fmt.Errorf("pipeline.ValidateHead: %w", err)
	}

	for _, proxyUrl := range pipeline.Head {
		_, ok := proxies[proxyUrl]
		if !ok {
			return fmt.Errorf("proxy '%s' url not required. call independent.RequireProxy", proxyUrl)
		}
	}

	if pipeline.End.IsController() {
		if err := controllers.Exist(pipeline.End.Id); err != nil {
			return fmt.Errorf("independent.Controllers.Exist('%s') [call independent.AddController()]: %w", pipeline.End.Id, err)
		}
	} else {
		if HasServicePipeline(pipelines) {
			return fmt.Errorf("configuration.HasServicePipeline: service pipeline added already")
		}
	}

	return nil
}
