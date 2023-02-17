package topic

import "github.com/blocklords/gosds/app/remote/message"

type TopicFilter struct {
	Organizations  []string
	Projects       []string
	NetworkIds     []string
	Groups         []string
	Smartcontracts []string
	Methods        []string
	Events         []string
}

func NewFilterTopic(o []string, p []string, n []string, g []string, s []string, m []string, e []string) TopicFilter {
	return TopicFilter{
		Organizations:  o,
		Projects:       p,
		NetworkIds:     n,
		Groups:         g,
		Smartcontracts: s,
		Methods:        m,
		Events:         e,
	}
}

func (t *TopicFilter) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"o":  t.Organizations,
		"p:": t.Projects,
		"n":  t.NetworkIds,
		"g":  t.Groups,
		"s":  t.Smartcontracts,
		"m":  t.Methods,
		"e":  t.Events,
	}
}

func (t *TopicFilter) Len(level uint8) int {
	switch level {
	case ORGANIZATION_LEVEL:
		return len(t.Organizations)
	case PROJECT_LEVEL:
		return len(t.Projects)
	case NETWORK_ID_LEVEL:
		return len(t.NetworkIds)
	case GROUP_LEVEL:
		return len(t.Groups)
	case SMARTCONTRACT_LEVEL:
		return len(t.Smartcontracts)
	case FULL_LEVEL:
		return len(t.Methods) + len(t.Events)
	default:
		return len(t.Organizations) + len(t.Projects) + len(t.NetworkIds) + len(t.Groups) + len(t.Smartcontracts) + len(t.Methods) + len(t.Events)
	}
}

// topic key
func (t *TopicFilter) Key() TopicKey {
	return TopicKey(t.ToString())
}

// list of path
func list(properties []string) string {
	str := ""
	for _, v := range properties {
		str += "," + v
	}

	return str
}

// Convert the topic filter object to the topic filter string.
func (t *TopicFilter) ToString() string {
	str := ""
	if len(t.Organizations) > 0 {
		str += "o:" + list(t.Organizations) + ";"
	}
	if len(t.Projects) > 0 {
		str += "p:" + list(t.Projects) + ";"
	}
	if len(t.NetworkIds) > 0 {
		str += "n:" + list(t.NetworkIds) + ";"
	}
	if len(t.Groups) > 0 {
		str += "g:" + list(t.Groups) + ";"
	}
	if len(t.Smartcontracts) > 0 {
		str += "s:" + list(t.Smartcontracts) + ";"
	}
	if len(t.Methods) > 0 {
		str += "m:" + list(t.Methods) + ";"
	}
	if len(t.Events) > 0 {
		str += "e:" + list(t.Events) + ";"
	}

	return str
}

// Converts the JSON object to the topic.TopicFilter
func ParseJSONToTopicFilter(parameters map[string]interface{}) (*TopicFilter, error) {
	topic_filter := TopicFilter{
		Organizations:  []string{},
		Projects:       []string{},
		NetworkIds:     []string{},
		Groups:         []string{},
		Smartcontracts: []string{},
		Methods:        []string{},
		Events:         []string{},
	}

	organizations, err := message.GetStringList(parameters, "o")
	if err == nil {
		topic_filter.Organizations = organizations
	}
	projects, err := message.GetStringList(parameters, "p")
	if err == nil {
		topic_filter.Projects = projects
	}
	network_ids, err := message.GetStringList(parameters, "n")
	if err == nil {
		topic_filter.NetworkIds = network_ids
	}
	groups, err := message.GetStringList(parameters, "g")
	if err == nil {
		topic_filter.Groups = groups
	}
	smartcontracts, err := message.GetStringList(parameters, "s")
	if err == nil {
		topic_filter.Smartcontracts = smartcontracts
	}
	methods, err := message.GetStringList(parameters, "m")
	if err == nil {
		topic_filter.Methods = methods
	}
	logs, err := message.GetStringList(parameters, "e")
	if err == nil {
		topic_filter.Events = logs
	}

	return &topic_filter, nil
}
