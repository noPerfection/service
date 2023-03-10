package configuration

// The Configuration sets the relationship between the organization and the smartcontract.
type Configuration struct {
	Organization string `json:"o"`
	Project      string `json:"p"`
	NetworkId    string `json:"n"`
	Group        string `json:"g"`
	Name         string `json:"s"`
	address      string
}

// The smartcontract address to which the configuration belongs too.
func (c *Configuration) SetAddress(address string) {
	c.address = address
}

func (c *Configuration) Address() string {
	return c.address
}
