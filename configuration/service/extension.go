package service

type Extension struct {
	Url  string
	Id   string
	Port uint64
}

// NewInternalExtension returns the extension that is on another thread, but not on client.
// The extension will be connected using the inproc protocol, not over TCP.
func NewInternalExtension(name string) *Extension {
	return &Extension{Url: name, Port: 0}
}
