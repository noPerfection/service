package identity

// Limit defines what kind of service is created.
// Depending on the limit the service will prepare the parameters for the
// client or for the controller.
type Limit uint8

const (
	REMOTE    Limit = 1 // Service is for the client socket. Service will include port, host and public key
	THIS      Limit = 2 // Service is for the controller. Service will include port, public key and secret key
	SUBSCRIBE Limit = 3 // Service is for client socket. Service will include port, host and public key for broadcast
	BROADCAST Limit = 4 // Service is for the controller. Service will include port, public key and secret key for broadcast
)
