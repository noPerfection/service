package service

import "fmt"

// ControllerType defines the available kind of controllers
type ControllerType string

// ReplierType or PublisherType or ReplierType
const (
	// SyncReplierType controllers are serving one request at a time. It's the handler in a
	// traditional client-handler model.
	SyncReplierType ControllerType = "SyncReplier"
	// PusherType controllers are serving the data to the Pullers without checking its delivery.
	// If multiple instances of Pullers are connected. Then Pusher sends the data to one Puller in a round-robin
	// way.
	PusherType ControllerType = "Pusher"
	// PublisherType controllers are broadcasting the message to all subscribers
	PublisherType ControllerType = "Publisher"
	// ReplierType controllers are the asynchronous ReplierType
	ReplierType ControllerType = "Replier"
	UnknownType ControllerType = ""
)

// ValidateControllerType checks whether the given string is the valid or not.
// If not valid, then returns the error otherwise returns nil.
func ValidateControllerType(t ControllerType) error {
	if t == SyncReplierType || t == PusherType || t == PublisherType || t == ReplierType {
		return nil
	}

	return fmt.Errorf("'%s' is not valid handler type", t)
}
