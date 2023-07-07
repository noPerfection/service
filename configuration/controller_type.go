package configuration

import "fmt"

// Type defines the available kind of controllers
type Type string

// Replier or Puller or Publisher or Router
const (
	// ReplierType controllers are serving one request at a time. It's the server in a
	// traditional client-server model.
	ReplierType Type = "Replier"
	// PusherType controllers are serving the data to the Pullers without checking its delivery.
	// If multiple instances of Pullers are connected. Then Pusher sends the data to one Puller in a round-robin
	// way.
	PusherType Type = "Pusher"
	// PublisherType controllers are broadcasting the message to all subscribers
	PublisherType Type = "Publisher"
	// RouterType controllers are the asynchronous Replier
	RouterType Type = "Router"
)

// ValidateControllerType checks whether the given string is the valid or not.
// If not valid then returns the error otherwise returns nil.
func ValidateControllerType(t Type) error {
	if t == ReplierType || t == PusherType || t == PublisherType || t == RouterType {
		return nil
	}

	return fmt.Errorf("'%s' is not valid controller type", t)
}
