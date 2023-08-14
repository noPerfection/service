package pipeline

type PipeEnd struct {
	Id  string
	Url string
}

// NewControllerPipeEnd creates a pipe end with the given name as the handler
func NewControllerPipeEnd(end string) *PipeEnd {
	return &PipeEnd{
		Url: "",
		Id:  end,
	}
}

func NewThisServicePipeEnd() *PipeEnd {
	return NewControllerPipeEnd("")
}

func (end *PipeEnd) IsController() bool {
	return len(end.Id) > 0
}

func (end *PipeEnd) Pipeline(head []string) *Pipeline {
	return &Pipeline{
		End:  end,
		Head: head,
	}
}
