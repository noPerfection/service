package topic

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/blocklords/sds/common/data_type/key_value"
)

type (
	TopicKey string
	Topic    struct {
		Organization  string `json:"o,omitempty"`
		Project       string `json:"p,omitempty"`
		NetworkId     string `json:"n,omitempty"`
		Group         string `json:"g,omitempty"`
		Smartcontract string `json:"s,omitempty"`
		Event         string `json:"e,omitempty"`
	}
)

func New(o string, p string, n string, g string, s string, e string) Topic {
	return Topic{
		Organization:  o,
		Project:       p,
		NetworkId:     n,
		Group:         g,
		Smartcontract: s,
		Event:         e,
	}
}

func (t *Topic) ToString(level uint8) string {
	if level < 1 || level > 6 {
		return ""
	}

	str := ""

	if level >= 1 {
		str += "o:" + t.Organization
	}
	if level >= 2 {
		str += ";p:" + t.Project
	}
	if level >= 3 {
		str += ";n:" + t.NetworkId
	}
	if level >= 4 {
		str += ";g:" + t.Group
	}
	if level >= 5 {
		str += ";s:" + t.Smartcontract
	}

	if level >= 6 {
		str += ";e:" + t.Event
	}

	return str
}

func (t *Topic) Level() uint8 {
	var level uint8 = 0
	if len(t.Organization) > 0 {
		level++

		if len(t.Project) > 0 {
			level++

			if len(t.NetworkId) > 0 {
				level++

				if len(t.Group) > 0 {
					level++

					if len(t.Smartcontract) > 0 {
						level++

						if len(t.Event) > 0 {
							level++
						}
					}
				}
			}
		}
	}
	return level
}

// Parse JSON into the Topic
func ParseJSON(parameters key_value.KeyValue) (*Topic, error) {
	organization, err := parameters.GetString("o")
	if err != nil {
		return nil, fmt.Errorf("parameters.GetString(`o`): %w", err)
	}
	if len(organization) == 0 {
		return nil, errors.New("empty 'o' parameter")
	}
	project, err := parameters.GetString("p")
	if err != nil {
		return nil, fmt.Errorf("parameters.GetString(`p`): %w", err)
	}
	if len(project) == 0 {
		return nil, errors.New("empty 'p' parameter")
	}
	topic := Topic{
		Organization:  organization,
		Project:       project,
		NetworkId:     "",
		Group:         "",
		Smartcontract: "",
		Event:         "",
	}

	network_id, err := parameters.GetString("n")
	if err == nil {
		topic.NetworkId = network_id
	}

	group, err := parameters.GetString("g")
	if err == nil {
		topic.Group = group
	}

	smartcontract, err := parameters.GetString("s")
	if err == nil {
		topic.Smartcontract = smartcontract
	}

	event, err := parameters.GetString("e")
	if err == nil {
		topic.Event = event
	}

	return &topic, nil
}

func isPathName(name string) bool {
	return name == "o" || name == "p" || name == "n" || name == "g" || name == "s" || name == "e"
}

func isLiteral(val string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9 _-]*$`).MatchString(val)
}

func (t *Topic) setNewValue(pathName string, val string) error {
	switch pathName {
	case "o":
		if len(t.Organization) > 0 {
			return fmt.Errorf("the duplicate organization path name. already set as " + t.Organization)
		} else {
			t.Organization = val
		}
	case "p":
		if len(t.Project) > 0 {
			return fmt.Errorf("the duplicate project path name. already set as " + t.Project)
		} else {
			t.Project = val
		}
	case "n":
		if len(t.NetworkId) > 0 {
			return fmt.Errorf("the duplicate network id path name. already set as " + t.NetworkId)
		} else {
			t.NetworkId = val
		}
	case "g":
		if len(t.Group) > 0 {
			return fmt.Errorf("the duplicate group path name. already set as " + t.Group)
		} else {
			t.Group = val
		}
	case "s":
		if len(t.Smartcontract) > 0 {
			return fmt.Errorf("the duplicate smartcontract path name. already set as " + t.Smartcontract)
		} else {
			t.Smartcontract = val
		}
	case "e":
		if len(t.Event) > 0 {
			return fmt.Errorf("the duplicate event path name. already set as " + t.Event)
		} else {
			t.Event = val
		}
	}

	return nil
}

// This method converts Topic String to the Topic Struct.
//
// The topic string is provided in the following string format:
//
//	`o:<organization>;p:<project>;n:<network id>;g:<group>;s:<smartcontract>;m:<method>`
//	`o:<organization>;p:<project>;n:<network id>;g:<group>;s:<smartcontract>;e:<event>`
//
// ----------------------
//
// Rules
//
//   - the topic string can have either `method` or `event` but not both at the same time.
//   - Topic string should contain atleast 'organization' and 'project'
//   - Order of the path names does not matter: o:org;p:proj == p:proj;o:org
//   - The values between `<` and `>` are literals and should return true by `isLiteral(literal)` function
func ParseString(topic_string string) (Topic, error) {
	parts := strings.Split(topic_string, ";")
	length := len(parts)
	if length < 2 {
		return Topic{}, fmt.Errorf("%s should have atleast 2 parts divided by ';'", topic_string)
	}

	if length > 6 {
		return Topic{}, fmt.Errorf("%s should have at most 6 parts divided by ';'", topic_string)
	}

	t := Topic{}

	for i, part := range parts {
		key_value := strings.Split(part, ":")
		if len(key_value) != 2 {
			return Topic{}, fmt.Errorf("part[%d] is %s, it can't be divided to two elements by ':'", i, part)
		}

		if !isPathName(key_value[0]) {
			return Topic{}, fmt.Errorf("part[%d] isPathName(%s) false", i, key_value[0])
		}

		if !isLiteral(key_value[1]) {
			return Topic{}, fmt.Errorf("part[%d] ('%s') isLiteral(%v) false", i, key_value[0], key_value[1])
		}

		err := t.setNewValue(key_value[0], key_value[1])
		if err != nil {
			return t, fmt.Errorf("part[%d] setNewValue: %w", i, err)
		}
	}

	return t, nil
}

const ORGANIZATION_LEVEL uint8 = 1  // only organization.
const PROJECT_LEVEL uint8 = 2       // only organization and project.
const NETWORK_ID_LEVEL uint8 = 3    // only organization, project and, network id.
const GROUP_LEVEL uint8 = 4         // only organization and project, network id and group.
const SMARTCONTRACT_LEVEL uint8 = 5 // smartcontract level path, till the smartcontract of the smartcontract
const FULL_LEVEL uint8 = 6          // full topic path
const ALL uint8 = 0                 // all, just like full, but full can be also only method|event.
