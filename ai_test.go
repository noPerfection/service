package service

import (
	"testing"

	"github.com/noPerfection/datatype"
	"github.com/stretchr/testify/require"
)

func TestAiParametersFromConfigUsesDefaults(t *testing.T) {
	apiKey, model, err := aiParametersFromConfig(Config{})
	require.NoError(t, err)
	require.Equal(t, defaultAiModel, model)
	require.Equal(t, "", apiKey)
}

func TestAiParametersFromConfigOverridesModel(t *testing.T) {
	apiKey, model, err := aiParametersFromConfig(Config{
		Parameters: datatype.New().Set(aiModelParameter, "custom-model"),
	})
	require.NoError(t, err)
	require.Equal(t, "custom-model", model)
	require.Equal(t, "", apiKey)
}

func TestAiParametersFromConfigReadsEmbeddedAPIKey(t *testing.T) {
	apiKey, model, err := aiParametersFromConfig(Config{
		Parameters: datatype.New().
			Set(aiAPIKeyParameter, "resolved-secret").
			Set(aiModelParameter, "claude-test"),
	})
	require.NoError(t, err)
	require.Equal(t, "resolved-secret", apiKey)
	require.Equal(t, "claude-test", model)
}

func TestNewAiServiceReadsParametersFromTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	configPath := DefaultConfigPath
	handler, err := newTopologyHandler(configPath)
	require.NoError(t, err)

	serviceConfig := defaultAiExtensionServiceConfig()
	serviceConfig.Parameters = datatype.New().
		Set(aiAPIKeyParameter, "test-key").
		Set(aiModelParameter, "claude-test")
	require.NoError(t, handler.AddService(serviceConfig))

	ai, err := NewAiService()
	require.NoError(t, err)
	require.NoError(t, ai.ensureTopologyHandler())
	require.NotNil(t, ai)
	model, err := ai.ensureProvider()
	require.NoError(t, err)
	require.Equal(t, "claude-test", model)
	require.NotNil(t, ai.provider)
}

func TestParseTwoCodeBlocks(t *testing.T) {
	content := "preamble\n```go\npackage foo\n```\n\n```go\npackage main\n```\n"
	first, second, err := parseTwoCodeBlocks(content)
	require.NoError(t, err)
	require.Equal(t, "package foo", first)
	require.Equal(t, "package main", second)
}
