package provider

import (
	"fmt"
	"net/url"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// parses JSON object into the Network Type
func New(data key_value.KeyValue) (Provider, error) {
	var provider Provider
	err := data.ToInterface(&provider)
	if err != nil {
		return provider, fmt.Errorf("failed to convert key-value to provider.Provider: %v", err)
	}

	if len(provider.Url) == 0 {
		return Provider{}, fmt.Errorf("empty url or its missing")
	}

	if provider.Length == 0 {
		return Provider{}, fmt.Errorf("length of the provider can not be zero")
	}
	if provider.Length > PROVIDER_MAX_LENGTH {
		return Provider{}, fmt.Errorf("the '%s' provider length '%d' exceeds the limit %d", provider.Url, provider.Length, PROVIDER_MAX_LENGTH)
	}

	u, err := url.ParseRequestURI(provider.Url)
	if err != nil {
		return Provider{}, fmt.Errorf("invalid '%s' provider url: %w", provider.Url, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Provider{}, fmt.Errorf("invalid '%s' provider protocol. Expected either 'http' or 'https'. But given '%s'", provider.Url, u.Scheme)
	}

	return provider, nil
}

// Create the list of providers from the given key value list
func NewList(datas []key_value.KeyValue) ([]Provider, error) {
	providers := make([]Provider, len(datas))

	for i, data := range datas {
		provider, err := New(data)
		if err != nil {
			return nil, fmt.Errorf("converting '%v' json to provider '%v';", data, err)
		}

		providers[i] = provider
	}

	return providers, nil
}
