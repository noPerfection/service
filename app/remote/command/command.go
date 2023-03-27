package command

type Command string

func (c Command) String() string {
	return string(c)
}
