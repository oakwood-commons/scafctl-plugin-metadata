// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"
	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProviders(t *testing.T) {
	p := NewPlugin()
	providers, err := p.GetProviders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{ProviderName}, providers)
}

func TestGetProviderDescriptor(t *testing.T) {
	p := NewPlugin()
	d, err := p.GetProviderDescriptor(context.Background(), ProviderName)
	require.NoError(t, err)
	assert.Equal(t, ProviderName, d.Name)
	assert.Equal(t, "Metadata Provider", d.DisplayName)
	assert.Equal(t, "v1", d.APIVersion)
	assert.NotNil(t, d.Schema)
	assert.Len(t, d.Capabilities, 1)
	assert.Equal(t, sdkprovider.CapabilityFrom, d.Capabilities[0])
	assert.Len(t, d.Examples, 1)
	assert.NotNil(t, d.OutputSchemas[sdkprovider.CapabilityFrom])
	assert.Equal(t, "Core", d.Category)
	assert.Contains(t, d.Tags, "metadata")
	assert.Contains(t, d.Tags, "introspection")
}

func TestGetProviderDescriptor_UnknownProvider(t *testing.T) {
	p := NewPlugin()
	_, err := p.GetProviderDescriptor(context.Background(), "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestConfigureProvider(t *testing.T) {
	p := NewPlugin()

	meta := hostMetadata{
		BuildVersion: "1.2.3",
		Commit:       "abc123",
		BuildTime:    "2025-01-01T00:00:00Z",
		Entrypoint:   "cli",
		Command:      "scafctl/run/solution",
		Args:         []string{"scafctl", "run", "solution"},
		Solution: solutionMeta{
			Name:        "my-solution",
			Version:     "1.0.0",
			DisplayName: "My Solution",
			Description: "A test solution",
			Category:    "testing",
			Tags:        []string{"test", "example"},
		},
	}
	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	err = p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": raw,
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "1.2.3", p.host.BuildVersion)
	assert.Equal(t, "abc123", p.host.Commit)
	assert.Equal(t, "cli", p.host.Entrypoint)
	assert.Equal(t, "my-solution", p.host.Solution.Name)
}

func TestConfigureProvider_UnknownProvider(t *testing.T) {
	p := NewPlugin()
	err := p.ConfigureProvider(context.Background(), "unknown", sdkplugin.ProviderConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestConfigureProvider_InvalidJSON(t *testing.T) {
	p := NewPlugin()
	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": json.RawMessage(`{invalid`),
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestConfigureProvider_NoMetadataKey(t *testing.T) {
	p := NewPlugin()
	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{},
	})
	require.NoError(t, err)
	// Host metadata should remain zero-valued.
	assert.Empty(t, p.host.BuildVersion)
}

func TestExecuteProvider_FullContext(t *testing.T) {
	p := NewPlugin()

	// Configure with full host metadata.
	meta := hostMetadata{
		BuildVersion: "1.2.3",
		Commit:       "abc123",
		BuildTime:    "2025-01-01T00:00:00Z",
		Entrypoint:   "cli",
		Command:      "scafctl/run/solution",
		Args:         []string{"scafctl", "run", "solution"},
		Solution: solutionMeta{
			Name:        "my-solution",
			Version:     "1.0.0",
			DisplayName: "My Solution",
			Description: "A test solution",
			Category:    "testing",
			Tags:        []string{"test", "example"},
		},
	}
	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	err = p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{"metadata": raw},
	})
	require.NoError(t, err)

	// Execute with working directory in context.
	ctx := sdkprovider.WithWorkingDirectory(context.Background(), "/test/cwd")
	out, err := p.ExecuteProvider(ctx, ProviderName, nil)
	require.NoError(t, err)

	result, ok := out.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any output")

	// Verify version info.
	versionMap, ok := result["version"].(map[string]any)
	require.True(t, ok, "version should be a map")
	assert.Equal(t, "1.2.3", versionMap["buildVersion"])
	assert.Equal(t, "abc123", versionMap["commit"])
	assert.Equal(t, "2025-01-01T00:00:00Z", versionMap["buildTime"])

	// Verify args from host.
	args, ok := result["args"].([]string)
	require.True(t, ok, "args should be []string")
	assert.Equal(t, []string{"scafctl", "run", "solution"}, args)

	// Verify cwd from context.
	assert.Equal(t, "/test/cwd", result["cwd"])

	// Verify entrypoint.
	assert.Equal(t, "cli", result["entrypoint"])

	// Verify command.
	assert.Equal(t, "scafctl/run/solution", result["command"])

	// Verify solution metadata.
	solMap, ok := result["solution"].(map[string]any)
	require.True(t, ok, "solution should be a map")
	assert.Equal(t, "my-solution", solMap["name"])
	assert.Equal(t, "1.0.0", solMap["version"])
	assert.Equal(t, "My Solution", solMap["displayName"])
	assert.Equal(t, "A test solution", solMap["description"])
	assert.Equal(t, "testing", solMap["category"])
	assert.Equal(t, []string{"test", "example"}, solMap["tags"])
}

func TestExecuteProvider_APIEntrypoint(t *testing.T) {
	p := NewPlugin()

	meta := hostMetadata{
		Entrypoint: "api",
		Command:    "api/v1/solutions/run",
	}
	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	err = p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{"metadata": raw},
	})
	require.NoError(t, err)

	out, err := p.ExecuteProvider(context.Background(), ProviderName, nil)
	require.NoError(t, err)

	result := out.Data.(map[string]any)
	assert.Equal(t, "api", result["entrypoint"])
	assert.Equal(t, "api/v1/solutions/run", result["command"])
}

func TestExecuteProvider_NoContext(t *testing.T) {
	p := NewPlugin()

	// No ConfigureProvider call -- host metadata is all zero-valued.
	out, err := p.ExecuteProvider(context.Background(), ProviderName, nil)
	require.NoError(t, err)

	result, ok := out.Data.(map[string]any)
	require.True(t, ok)

	// Entrypoint should be "unknown" when not configured.
	assert.Equal(t, "unknown", result["entrypoint"])
	assert.Equal(t, "", result["command"])

	// Solution should be an empty map.
	solMap, ok := result["solution"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, solMap)

	// Version fields should be empty strings.
	versionMap, ok := result["version"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "", versionMap["buildVersion"])

	// Args should fall back to os.Args.
	args, ok := result["args"].([]string)
	require.True(t, ok)
	assert.Equal(t, os.Args, args)

	// CWD should fall back to os.Getwd().
	expectedCwd, _ := os.Getwd()
	assert.Equal(t, expectedCwd, result["cwd"])
}

func TestExecuteProvider_NoSolutionMetadata(t *testing.T) {
	p := NewPlugin()

	meta := hostMetadata{
		Entrypoint: "cli",
		Command:    "scafctl/run/resolver",
	}
	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	err = p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{"metadata": raw},
	})
	require.NoError(t, err)

	out, err := p.ExecuteProvider(context.Background(), ProviderName, nil)
	require.NoError(t, err)

	result := out.Data.(map[string]any)
	assert.Equal(t, "cli", result["entrypoint"])

	// Solution should be an empty map when not set.
	solMap, ok := result["solution"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, solMap)
}

func TestExecuteProvider_UnknownProvider(t *testing.T) {
	p := NewPlugin()
	_, err := p.ExecuteProvider(context.Background(), "unknown", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestDescribeWhatIf(t *testing.T) {
	p := NewPlugin()
	desc, err := p.DescribeWhatIf(context.Background(), ProviderName, nil)
	require.NoError(t, err)
	assert.Contains(t, desc, "runtime metadata")
}

func TestDescribeWhatIf_UnknownProvider(t *testing.T) {
	p := NewPlugin()
	_, err := p.DescribeWhatIf(context.Background(), "unknown", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestExecuteProviderStream_NotSupported(t *testing.T) {
	p := NewPlugin()
	err := p.ExecuteProviderStream(context.Background(), ProviderName, nil, nil)
	assert.ErrorIs(t, err, sdkplugin.ErrStreamingNotSupported)
}

func TestExtractDependencies(t *testing.T) {
	p := NewPlugin()
	deps, err := p.ExtractDependencies(context.Background(), ProviderName, nil)
	require.NoError(t, err)
	assert.Nil(t, deps)
}

func TestStopProvider(t *testing.T) {
	p := NewPlugin()
	err := p.StopProvider(context.Background(), ProviderName)
	require.NoError(t, err)
}

func TestPluginInterface(t *testing.T) {
	// Verify Plugin satisfies the ProviderPlugin interface at compile time.
	var _ sdkplugin.ProviderPlugin = (*Plugin)(nil)
}
