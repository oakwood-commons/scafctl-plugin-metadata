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
