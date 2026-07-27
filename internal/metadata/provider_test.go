// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
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
	assert.Contains(t, d.Tags, "platform")
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

	// Verify platform info.
	assert.Equal(t, runtime.GOOS, result["os"])
	assert.Equal(t, runtime.GOARCH, result["arch"])
	assert.NotNil(t, result["shell"]) // shell is always set (may be empty string)
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

	// os and arch should always be populated.
	assert.Equal(t, runtime.GOOS, result["os"])
	assert.Equal(t, runtime.GOARCH, result["arch"])
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

// marshalMetadata is a test helper that JSON-encodes host metadata.
func marshalMetadata(t *testing.T, meta hostMetadata) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	return raw
}

// TestExecuteProvider_PerExecutionSettings simulates a pool-mode host that
// delivers stale/empty config via ConfigureProvider but supplies the correct,
// fresh metadata per execution via context. The per-execution settings must win.
func TestExecuteProvider_PerExecutionSettings(t *testing.T) {
	p := NewPlugin()

	// Configure with stale/empty host metadata (pool-mode load-time config).
	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": marshalMetadata(t, hostMetadata{}),
		},
	})
	require.NoError(t, err)

	// Per-execution settings carry the correct values for this solution.
	perExec := hostMetadata{
		BuildVersion: "9.9.9",
		Commit:       "def456",
		BuildTime:    "2026-07-27T00:00:00Z",
		Entrypoint:   "api",
		Command:      "api/v1/solutions/run",
		Args:         []string{"scafctl-api"},
		Solution: solutionMeta{
			Name:    "pool-solution",
			Version: "2.0.0",
			Source:  "./pool-solution.yaml",
		},
	}
	ctx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
		"metadata": marshalMetadata(t, perExec),
	})

	out, err := p.ExecuteProvider(ctx, ProviderName, nil)
	require.NoError(t, err)

	result := out.Data.(map[string]any)

	versionMap := result["version"].(map[string]any)
	assert.Equal(t, "9.9.9", versionMap["buildVersion"])
	assert.Equal(t, "def456", versionMap["commit"])
	assert.Equal(t, []string{"scafctl-api"}, result["args"])
	assert.Equal(t, "api", result["entrypoint"])
	assert.Equal(t, "api/v1/solutions/run", result["command"])

	solMap := result["solution"].(map[string]any)
	assert.Equal(t, "pool-solution", solMap["name"])
	assert.Equal(t, "2.0.0", solMap["version"])
	assert.Equal(t, "./pool-solution.yaml", solMap["source"])
}

// TestExecuteProvider_PerExecutionSettingsOverrideConfigure ensures that when
// both a ConfigureProvider copy and per-execution settings are present, the
// per-execution values take precedence.
func TestExecuteProvider_PerExecutionSettingsOverrideConfigure(t *testing.T) {
	p := NewPlugin()

	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": marshalMetadata(t, hostMetadata{
				BuildVersion: "1.0.0",
				Entrypoint:   "cli",
				Command:      "scafctl/run/solution",
			}),
		},
	})
	require.NoError(t, err)

	ctx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
		"metadata": marshalMetadata(t, hostMetadata{
			BuildVersion: "2.0.0",
			Entrypoint:   "api",
			Command:      "api/v1/solutions/run",
		}),
	})

	out, err := p.ExecuteProvider(ctx, ProviderName, nil)
	require.NoError(t, err)

	result := out.Data.(map[string]any)
	assert.Equal(t, "2.0.0", result["version"].(map[string]any)["buildVersion"])
	assert.Equal(t, "api", result["entrypoint"])
	assert.Equal(t, "api/v1/solutions/run", result["command"])
}

// TestExecuteProvider_MalformedPerExecutionSettings verifies the provider falls
// back to the stored ConfigureProvider copy when per-execution settings are
// present but not valid JSON.
func TestExecuteProvider_MalformedPerExecutionSettings(t *testing.T) {
	p := NewPlugin()

	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": marshalMetadata(t, hostMetadata{
				BuildVersion: "1.0.0",
				Entrypoint:   "cli",
			}),
		},
	})
	require.NoError(t, err)

	ctx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
		"metadata": json.RawMessage(`{invalid`),
	})

	out, err := p.ExecuteProvider(ctx, ProviderName, nil)
	require.NoError(t, err)

	result := out.Data.(map[string]any)
	assert.Equal(t, "1.0.0", result["version"].(map[string]any)["buildVersion"])
	assert.Equal(t, "cli", result["entrypoint"])
}

// TestExecuteProvider_EmptyPerExecutionSettings verifies the provider falls back
// to the stored ConfigureProvider copy when the per-execution payload is present
// but empty (null / {} / all zero values), rather than blanking out valid config.
func TestExecuteProvider_EmptyPerExecutionSettings(t *testing.T) {
	cfg := hostMetadata{
		BuildVersion: "1.0.0",
		Entrypoint:   "cli",
		Command:      "scafctl/run/solution",
		Solution: solutionMeta{
			Name:    "cfg-solution",
			Version: "1.0.0",
			Source:  "./cfg-solution.yaml",
		},
	}

	for _, tc := range []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "null", raw: json.RawMessage(`null`)},
		{name: "empty object", raw: json.RawMessage(`{}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPlugin()
			require.NoError(t, p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
				Settings: map[string]json.RawMessage{"metadata": marshalMetadata(t, cfg)},
			}))

			ctx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
				"metadata": tc.raw,
			})

			out, err := p.ExecuteProvider(ctx, ProviderName, nil)
			require.NoError(t, err)

			result := out.Data.(map[string]any)
			assert.Equal(t, "1.0.0", result["version"].(map[string]any)["buildVersion"])
			assert.Equal(t, "cli", result["entrypoint"])
			assert.Equal(t, "scafctl/run/solution", result["command"])

			solMap := result["solution"].(map[string]any)
			assert.Equal(t, "cfg-solution", solMap["name"])
			assert.Equal(t, "./cfg-solution.yaml", solMap["source"])
		})
	}
}
// context settings are present but carry no "metadata" key, the provider falls
// back to the stored ConfigureProvider copy.
func TestExecuteProvider_PerExecutionSettingsWithoutMetadataKey(t *testing.T) {
	p := NewPlugin()

	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": marshalMetadata(t, hostMetadata{BuildVersion: "1.0.0"}),
		},
	})
	require.NoError(t, err)

	ctx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
		"other": json.RawMessage(`{}`),
	})

	out, err := p.ExecuteProvider(ctx, ProviderName, nil)
	require.NoError(t, err)

	result := out.Data.(map[string]any)
	assert.Equal(t, "1.0.0", result["version"].(map[string]any)["buildVersion"])
}

// TestExecuteProvider_SolutionMetadataFromContext verifies the canonical
// per-execution solution metadata channel is preferred over the solution
// embedded in the host settings, and that the source field is surfaced.
func TestExecuteProvider_SolutionMetadataFromContext(t *testing.T) {
	p := NewPlugin()

	// Host settings carry a different (stale) solution.
	ctx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
		"metadata": marshalMetadata(t, hostMetadata{
			Entrypoint: "cli",
			Solution: solutionMeta{
				Name:    "stale-solution",
				Version: "0.0.1",
			},
		}),
	})

	// Canonical solution metadata from context should win.
	ctx = sdkprovider.WithSolutionMetadata(ctx, &sdkprovider.SolutionMeta{
		Name:        "fresh-solution",
		Version:     "3.0.0",
		DisplayName: "Fresh Solution",
		Description: "The current solution",
		Category:    "infrastructure",
		Tags:        []string{"a", "b"},
		Source:      "./fresh-solution.yaml",
	})

	out, err := p.ExecuteProvider(ctx, ProviderName, nil)
	require.NoError(t, err)

	solMap := out.Data.(map[string]any)["solution"].(map[string]any)
	assert.Equal(t, "fresh-solution", solMap["name"])
	assert.Equal(t, "3.0.0", solMap["version"])
	assert.Equal(t, "Fresh Solution", solMap["displayName"])
	assert.Equal(t, "The current solution", solMap["description"])
	assert.Equal(t, "infrastructure", solMap["category"])
	assert.Equal(t, []string{"a", "b"}, solMap["tags"])
	assert.Equal(t, "./fresh-solution.yaml", solMap["source"])
}

// TestExecuteProvider_SolutionSourceFromConfigure verifies the source field is
// surfaced from the stored ConfigureProvider copy when no context solution
// metadata is present (older host).
func TestExecuteProvider_SolutionSourceFromConfigure(t *testing.T) {
	p := NewPlugin()

	err := p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{
			"metadata": marshalMetadata(t, hostMetadata{
				Solution: solutionMeta{
					Name:    "cfg-solution",
					Version: "1.0.0",
					Source:  "./cfg-solution.yaml",
				},
			}),
		},
	})
	require.NoError(t, err)

	out, err := p.ExecuteProvider(context.Background(), ProviderName, nil)
	require.NoError(t, err)

	solMap := out.Data.(map[string]any)["solution"].(map[string]any)
	assert.Equal(t, "cfg-solution", solMap["name"])
	assert.Equal(t, "./cfg-solution.yaml", solMap["source"])
}

// TestExecuteProvider_PoolAndPerCallIdentical asserts that identical host data
// delivered via ConfigureProvider (per-call host) and via context
// (pool-mode host) produces identical output.
func TestExecuteProvider_PoolAndPerCallIdentical(t *testing.T) {
	meta := hostMetadata{
		BuildVersion: "1.2.3",
		Commit:       "abc123",
		BuildTime:    "2026-01-01T00:00:00Z",
		Entrypoint:   "cli",
		Command:      "scafctl/run/solution",
		Args:         []string{"scafctl", "run", "solution"},
		Solution: solutionMeta{
			Name:    "shared-solution",
			Version: "1.0.0",
			Source:  "./shared.yaml",
		},
	}

	// Per-call host: data via ConfigureProvider, executed with a bare context.
	perCall := NewPlugin()
	require.NoError(t, perCall.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{"metadata": marshalMetadata(t, meta)},
	}))
	perCallOut, err := perCall.ExecuteProvider(context.Background(), ProviderName, nil)
	require.NoError(t, err)

	// Pool-mode host: empty config, data via per-execution context.
	pool := NewPlugin()
	require.NoError(t, pool.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{"metadata": marshalMetadata(t, hostMetadata{})},
	}))
	poolCtx := sdkprovider.WithSettings(context.Background(), map[string]json.RawMessage{
		"metadata": marshalMetadata(t, meta),
	})
	poolOut, err := pool.ExecuteProvider(poolCtx, ProviderName, nil)
	require.NoError(t, err)

	assert.Equal(t, perCallOut.Data, poolOut.Data)
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

func TestPluginInterface(_ *testing.T) {
	var _ sdkplugin.ProviderPlugin = (*Plugin)(nil)
}

// ── Shell detection tests ──────────────────────────────────────────────────

func TestDetectShell_FromSHELL(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	assert.Equal(t, "zsh", detectShell())
}

func TestDetectShell_FromComSpec(t *testing.T) {
	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("ComSpec", "/c/Windows/system32/cmd.exe")
	assert.Equal(t, "cmd.exe", detectShell())
}

func TestDetectShell_Empty(t *testing.T) {
	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("ComSpec", "")
	assert.Equal(t, "", detectShell())
}

func TestDetectShell_SHELLTakesPrecedence(t *testing.T) {
	// $SHELL should win even if PSModulePath and ComSpec are set.
	t.Setenv("SHELL", "/usr/bin/bash")
	t.Setenv("PSModulePath", "C:\\modules")
	t.Setenv("ComSpec", "C:\\Windows\\system32\\cmd.exe")
	assert.Equal(t, "bash", detectShell())
}

func TestDetectShell_PSModulePath_Pwsh(t *testing.T) {
	origGoos := goosFunc
	origParent := parentProcessNameFunc
	t.Cleanup(func() {
		goosFunc = origGoos
		parentProcessNameFunc = origParent
	})

	goosFunc = func() string { return "windows" }
	parentProcessNameFunc = func() string { return "pwsh.exe" }

	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", `C:\Users\test\Documents\PowerShell\Modules`)
	t.Setenv("ComSpec", `C:\Windows\system32\cmd.exe`)

	assert.Equal(t, "pwsh", detectShell())
}

func TestDetectShell_PSModulePath_WindowsPowerShell(t *testing.T) {
	origGoos := goosFunc
	origParent := parentProcessNameFunc
	t.Cleanup(func() {
		goosFunc = origGoos
		parentProcessNameFunc = origParent
	})

	goosFunc = func() string { return "windows" }
	parentProcessNameFunc = func() string { return "powershell.exe" }

	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", `C:\Users\test\Documents\PowerShell\Modules`)
	t.Setenv("ComSpec", `C:\Windows\system32\cmd.exe`)

	assert.Equal(t, "powershell", detectShell())
}

func TestDetectShell_GitBashOnWindows(t *testing.T) {
	// Git Bash sets $SHELL, so it takes precedence.
	t.Setenv("SHELL", "/usr/bin/bash")
	t.Setenv("PSModulePath", "")
	t.Setenv("ComSpec", `C:\Windows\system32\cmd.exe`)

	assert.Equal(t, "bash", detectShell())
}

func TestDetectShell_CmdExeFallback(t *testing.T) {
	origGoos := goosFunc
	t.Cleanup(func() { goosFunc = origGoos })

	goosFunc = func() string { return "windows" }

	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", "")
	// Use forward slashes so filepath.Base works correctly on all platforms.
	t.Setenv("ComSpec", "C:/Windows/system32/cmd.exe")

	assert.Equal(t, "cmd.exe", detectShell())
}

func TestDetectPowerShellVariant_Pwsh(t *testing.T) {
	orig := parentProcessNameFunc
	t.Cleanup(func() { parentProcessNameFunc = orig })

	parentProcessNameFunc = func() string { return "pwsh.exe" }
	assert.Equal(t, "pwsh", detectPowerShellVariant())
}

func TestDetectPowerShellVariant_WindowsPowerShell(t *testing.T) {
	orig := parentProcessNameFunc
	t.Cleanup(func() { parentProcessNameFunc = orig })

	parentProcessNameFunc = func() string { return "powershell.exe" }
	assert.Equal(t, "powershell", detectPowerShellVariant())
}

func TestDetectPowerShellVariant_Unknown(t *testing.T) {
	orig := parentProcessNameFunc
	t.Cleanup(func() { parentProcessNameFunc = orig })

	parentProcessNameFunc = func() string { return "" }
	assert.Equal(t, "pwsh", detectPowerShellVariant())
}

func TestDetectPowerShellVariant_UnexpectedParent(t *testing.T) {
	orig := parentProcessNameFunc
	t.Cleanup(func() { parentProcessNameFunc = orig })

	parentProcessNameFunc = func() string { return "explorer.exe" }
	assert.Equal(t, "pwsh", detectPowerShellVariant())
}
