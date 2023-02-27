package provider

import (
	"errors"
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// parses JSON object into the Network Type
func New(data key_value.KeyValue) (Provider, error) {
	var provider Provider
	err := data.ToInterface(&provider)
	if err != nil {
		return provider, fmt.Errorf("failed to serialize key-value to provider.Provider: %v", err)
	}

	if provider.Length == 0 {
		return Provider{}, errors.New("length of the provider can not be zero")
	}

	return provider, nil
}

// Create the list of providers from the given key value list
func NewList(datas []key_value.KeyValue) ([]Provider, error) {
	providers := make([]Provider, 0, len(datas))

	for i, data := range datas {
		provider, err := New(data)
		if err != nil {
			return nil, err
		}

		providers[i] = provider
	}

	return providers, nil
}
