package pipeline

func HasServicePipeline(pipelines []Pipeline) bool {
	for _, pipeline := range pipelines {
		if !pipeline.End.IsController() {
			return true
		}
	}

	return false
}

func GetControllerEnds(allPipelines []Pipeline) []Pipeline {
	pipelines := make([]Pipeline, 0, len(allPipelines))
	count := 0

	for _, pipeline := range allPipelines {
		if pipeline.End.IsController() {
			pipelines[count] = pipeline
			count++
		}
	}

	return pipelines
}

func GetServiceEnd(allPipelines []Pipeline) *Pipeline {
	for _, pipeline := range allPipelines {
		if !pipeline.End.IsController() {
			return &pipeline
		}
	}

	return nil
}
