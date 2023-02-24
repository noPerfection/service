package provider

type Provider struct {
	Url    string `json:"url"`
	Length uint64 `json:"length"` // How many blocks we can fetch from this provider
}
