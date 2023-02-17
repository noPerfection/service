package topic

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/blocklords/gosds/app/remote/message"
)

type (
	TopicKey string
	Topic    struct {
		Organization  string
		Project       string
		NetworkId     string
		Group         string
		Smartcontract string
		Method        string
		Event         string
	}
)

func New(o string, p string, n string, g string, s string, m string, e string) Topic {
	return Topic{
		Organization:  o,
		Project:       p,
		NetworkId:     n,
		Group:         g,
		Smartcontract: s,
		Method:        m,
		Event:         e,
	}
}

func (t *Topic) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"o": t.Organization,
		"p": t.Project,
		"n": t.NetworkId,
		"g": t.Group,
		"s": t.Smartcontract,
		"m": t.Method,
		"e": t.Event,
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
	if level == 6 {
		if len(t.Method) > 0 {
			str += ";m:" + t.Method
		} else if len(t.Event) > 0 {
			str += ";e:" + t.Event
		}
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

						if len(t.Method) > 0 || len(t.Event) > 0 {
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
func ParseJSON(parameters map[string]interface{}) (*Topic, error) {
	organization, err := message.GetString(parameters, "o")
	if err != nil {
		return nil, err
	}
	if len(organization) == 0 {
		return nil, errors.New("organization is empty")
	}
	project, err := message.GetString(parameters, "p")
	if err != nil {
		return nil, err
	}
	if len(project) == 0 {
		return nil, errors.New("project is empty")
	}
	topic := Topic{
		Organization:  organization,
		Project:       project,
		NetworkId:     "",
		Group:         "",
		Smartcontract: "",
		Method:        "",
		Event:         "",
	}

	network_id, err := message.GetString(parameters, "n")
	if err == nil {
		topic.NetworkId = network_id
	}

	group, err := message.GetString(parameters, "g")
	if err == nil {
		topic.Group = group
	}

	smartcontract, err := message.GetString(parameters, "s")
	if err == nil {
		topic.Smartcontract = smartcontract
	}

	method, err := message.GetString(parameters, "m")
	if err == nil {
		topic.Method = method
	}

	event, err := message.GetString(parameters, "e")
	if err == nil {
		topic.Event = event
	}

	return &topic, nil
}

func isPathName(name string) bool {
	return name == "o" || name == "p" || name == "n" || name == "g" || name == "s" || name == "m" || name == "e"
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
	case "m":
		if len(t.Method) > 0 {
			return fmt.Errorf("the duplicate method path name. already set as " + t.Method)
		} else {
			t.Method = val
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
func ParseString(topicString string) (Topic, error) {
	parts := strings.Split(topicString, ";")
	length := len(parts)
	if length < 2 {
		return Topic{}, fmt.Errorf("path should have atleast two elements")
	}

	if length > 6 {
		return Topic{}, fmt.Errorf("at most topic string can have six path names")
	}

	t := Topic{}

	for _, part := range parts {
		keyValue := strings.Split(part, ":")
		if len(keyValue) != 2 {
			return Topic{}, fmt.Errorf("invalid key:value in the topic string")
		}

		if !isPathName(keyValue[0]) {
			return Topic{}, fmt.Errorf("invalid path name: %s", keyValue[0])
		}

		if !isLiteral(keyValue[1]) {
			return Topic{}, fmt.Errorf("invalid literal for path name '%s': %s", keyValue[0], keyValue[1])
		}

		err := t.setNewValue(keyValue[0], keyValue[1])
		if err != nil {
			return t, err
		}
	}

	if len(t.Method) > 0 && len(t.Event) > 0 {
		return Topic{}, fmt.Errorf("only 'method' path name or 'event' path name can be set, but not at the same time")
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
