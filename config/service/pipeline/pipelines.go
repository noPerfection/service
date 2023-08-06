package pipeline

func FindControllerEnds(allPipelines []*Pipeline) []*Pipeline {
	pipelines := make([]*Pipeline, 0, len(allPipelines))
	count := 0

	for _, pipeline := range allPipelines {
		if pipeline.End.IsController() {
			pipelines[count] = pipeline
			count++
		}
	}

	return pipelines
}

func FindServiceEnd(allPipelines []*Pipeline) *Pipeline {
	for _, pipeline := range allPipelines {
		if !pipeline.End.IsController() {
			return pipeline
		}
	}

	return nil
}
