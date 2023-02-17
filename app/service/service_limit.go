package service

type Limit uint8

const (
	REMOTE    Limit = 1 // requires: port, host and public key
	THIS      Limit = 2 // requires: port, public key and secret key
	SUBSCRIBE Limit = 3 // requires: port, host and public key for broadcast
	BROADCAST Limit = 4 // requires: port, public key and secret key for broadcast
)
