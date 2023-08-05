package service

import (
	"fmt"
	"strings"
)

type Pipeline struct {
	End  *PipeEnd
	Head []string
}

// ValidateHead makes sure that pipeline has proxies, and they are all proxies are unique
func (pipeline *Pipeline) ValidateHead() error {
	if !pipeline.HasBeginning() {
		return fmt.Errorf("no head")
	}
	last := len(pipeline.Head) - 1
	for i := 0; i < last; i++ {
		needle := pipeline.Head[i]

		for j := i + 1; j <= last; j++ {
			url := pipeline.Head[j]
			if strings.Compare(url, needle) == 0 {
				return fmt.Errorf("the %d and %d proxies in the head are duplicates", i, j)
			}
		}
	}

	return nil
}

func (pipeline *Pipeline) HasLength() bool {
	return pipeline.End != nil && pipeline.HasBeginning()
}

func (pipeline *Pipeline) IsMultiHead() bool {
	return len(pipeline.Head) > 1
}

// HeadFront returns all proxy except the last one.
func (pipeline *Pipeline) HeadFront() []string {
	return pipeline.Head[:len(pipeline.Head)-1]
}

// HeadLast returns the last proxy in the proxy chain
func (pipeline *Pipeline) HeadLast() string {
	return pipeline.Head[len(pipeline.Head)-1]
}

// Beginning Returns the first proxy url in the pipeline. Doesn't validate it.
// Call first HasBeginning to check does the pipeline have a beginning
func (pipeline *Pipeline) Beginning() string {
	return pipeline.Head[0]
}

func (pipeline *Pipeline) HasBeginning() bool {
	return len(pipeline.Head) > 0
}
