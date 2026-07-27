// Package metadata implements the metadata provider plugin.
//
// The metadata provider returns runtime metadata about the scafctl host process
// and the currently-executing solution. It requires no inputs -- all data is
// gathered from the plugin configuration and execution context.
//
// Host-side data (version, entrypoint, args, solution metadata) is received via
// ConfigureProvider and stored in the plugin. Under pool-mode hosts that copy is
// stale, so ExecuteProvider prefers the effective per-execution host settings
// and solution metadata delivered via the SDK context accessors, falling back to
// the stored copy for older hosts. Per-execution data (working directory) comes
// from the SDK context helpers.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/google/jsonschema-go/jsonschema"
	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"
	sdkprovider "github.com/oakwood-commons/scafctl-plugin-sdk/provider"
	sdkhelper "github.com/oakwood-commons/scafctl-plugin-sdk/provider/schemahelper"
)

const (
	// ProviderName is the unique identifier for this provider.
	ProviderName = "metadata"

	// Version is the provider version.
	Version = "3.2.0"
)

// parentProcessNameFunc returns the parent process executable name.
// Points to the platform-specific parentProcessName by default.
// Overridable for testing.
var parentProcessNameFunc = parentProcessName

// goosFunc returns the current OS. Overridable for testing.
var goosFunc = func() string { return runtime.GOOS }

// hostMetadata holds host-supplied information received via ConfigureProvider.
type hostMetadata struct {
	// Version info from the host binary.
	BuildVersion string `json:"buildVersion"`
	Commit       string `json:"commit"`
	BuildTime    string `json:"buildTime"`

	// Entrypoint info.
	Entrypoint string `json:"entrypoint"` // "cli", "api", or "unknown"
	Command    string `json:"command"`    // e.g. "scafctl/run/solution"

	// Host CLI arguments.
	Args []string `json:"args"`

	// Solution metadata from the host.
	Solution solutionMeta `json:"solution"`
}

// solutionMeta mirrors the host's SolutionMeta.
type solutionMeta struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Source      string   `json:"source"`
}

// Plugin implements the scafctl ProviderPlugin interface.
type Plugin struct {
	host hostMetadata
}

// NewPlugin creates a new metadata plugin instance.
func NewPlugin() *Plugin {
	return &Plugin{}
}

// GetProviders returns the list of providers exposed by this plugin.
//
//nolint:revive // ctx required by interface
func (p *Plugin) GetProviders(_ context.Context) ([]string, error) {
	return []string{ProviderName}, nil
}

// GetProviderDescriptor returns the descriptor for the named provider.
//
//nolint:revive // ctx required by interface
func (p *Plugin) GetProviderDescriptor(_ context.Context, providerName string) (*sdkprovider.Descriptor, error) {
	if providerName != ProviderName {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	return &sdkprovider.Descriptor{
		Name:        ProviderName,
		DisplayName: "Metadata Provider",
		Description: "Returns runtime metadata about the scafctl process and the currently-executing solution. " +
			"Provides the scafctl version, CLI arguments, working directory, entrypoint type (cli/api), " +
			"command path, solution metadata, and platform information (os, arch), and the user's default shell. Requires no inputs.",
		APIVersion: "v1",
		Version:    semver.MustParse(Version),
		Category:   "Core",
		Tags:       []string{"metadata", "solution", "introspection", "runtime", "platform"},
		Capabilities: []sdkprovider.Capability{
			sdkprovider.CapabilityFrom,
		},
		Schema: sdkhelper.ObjectSchema(nil, map[string]*jsonschema.Schema{}),
		OutputSchemas: map[sdkprovider.Capability]*jsonschema.Schema{
			sdkprovider.CapabilityFrom: sdkhelper.ObjectSchema(
				[]string{"version", "args", "cwd", "entrypoint", "command", "solution", "os", "arch", "shell"},
				map[string]*jsonschema.Schema{
					"version": sdkhelper.ObjectProp("Build version information", nil, map[string]*jsonschema.Schema{
						"buildVersion": sdkhelper.StringProp("Semantic version of the scafctl build"),
						"commit":       sdkhelper.StringProp("Git commit hash of the build"),
						"buildTime":    sdkhelper.StringProp("Timestamp of the build"),
					}),
					"args":       sdkhelper.ArrayProp("Command-line arguments passed to scafctl", sdkhelper.WithItems(sdkhelper.StringProp("A CLI argument"))),
					"cwd":        sdkhelper.StringProp("Current working directory"),
					"entrypoint": sdkhelper.StringProp("How scafctl was invoked", sdkhelper.WithEnum("cli", "api", "unknown")),
					"command":    sdkhelper.StringProp("The command path (e.g. scafctl/run/solution)"),
					"solution": sdkhelper.ObjectProp("Metadata about the currently-running solution", nil, map[string]*jsonschema.Schema{
						"name":        sdkhelper.StringProp("Solution name"),
						"version":     sdkhelper.StringProp("Solution version"),
						"displayName": sdkhelper.StringProp("Solution display name"),
						"description": sdkhelper.StringProp("Solution description"),
						"category":    sdkhelper.StringProp("Solution category"),
						"tags":        sdkhelper.ArrayProp("Solution tags", sdkhelper.WithItems(sdkhelper.StringProp("A tag"))),
						"source":      sdkhelper.StringProp("Where the solution was loaded from (file path, catalog reference, or URL)"),
					}),
					"os":    sdkhelper.StringProp("Operating system (runtime.GOOS)", sdkhelper.WithEnum("aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "js", "linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows")),
					"arch":  sdkhelper.StringProp("CPU architecture (runtime.GOARCH)", sdkhelper.WithEnum("386", "amd64", "arm", "arm64", "loong64", "mips", "mips64", "mips64le", "mipsle", "ppc64", "ppc64le", "riscv64", "s390x", "wasm")),
					"shell": sdkhelper.StringProp("User's shell (from $SHELL on Unix; PSModulePath/parent-process heuristic on Windows; %ComSpec% fallback)"),
				},
			),
		},
		Examples: []sdkprovider.Example{
			{
				Name:        "Runtime metadata",
				Description: "Return all runtime metadata about the scafctl process and current solution",
				YAML:        "name: runtime-meta\ntype: metadata\nfrom:\n  inputs: {}",
			},
		},
	}, nil
}

// ConfigureProvider stores host-side configuration including version info,
// entrypoint, and solution metadata.
func (p *Plugin) ConfigureProvider(_ context.Context, providerName string, cfg sdkplugin.ProviderConfig) error {
	if providerName != ProviderName {
		return fmt.Errorf("unknown provider: %s", providerName)
	}

	// Parse host metadata from settings.
	if raw, ok := cfg.Settings["metadata"]; ok {
		if err := json.Unmarshal(raw, &p.host); err != nil {
			return fmt.Errorf("failed to unmarshal host metadata: %w", err)
		}
	}

	return nil
}

// ExecuteProvider gathers runtime metadata from the host configuration and
// execution context.
//
// Under a pool-mode host (MCP/API server) the plugin process is configured once
// at load time, so the copy stored via ConfigureProvider is stale. The SDK
// therefore delivers the effective per-execution host settings via
// sdkprovider.SettingsFromContext and canonical solution metadata via
// sdkprovider.SolutionMetadataFromContext. This method prefers those
// context-scoped values and falls back to the ConfigureProvider copy when they
// are absent (older hosts), so output is identical under pool-mode and per-call
// hosts.
func (p *Plugin) ExecuteProvider(ctx context.Context, providerName string, _ map[string]any) (*sdkprovider.Output, error) {
	if providerName != ProviderName {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	lgr := logr.FromContextOrDiscard(ctx)
	lgr.V(1).Info("executing provider", "provider", ProviderName)

	// Resolve the effective host metadata: prefer per-execution settings
	// delivered via context, falling back to the ConfigureProvider copy.
	host := p.effectiveHost(ctx)

	// Build version info from host config.
	version := map[string]any{
		"buildVersion": host.BuildVersion,
		"commit":       host.Commit,
		"buildTime":    host.BuildTime,
	}

	// CLI arguments from host config.
	args := host.Args
	if args == nil {
		args = os.Args // Fallback to plugin process args if host didn't send them.
	}

	// Current working directory (context-aware).
	cwd, ok := sdkprovider.WorkingDirectoryFromContext(ctx)
	if !ok || cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Entrypoint and command path from host config.
	entrypoint := host.Entrypoint
	if entrypoint == "" {
		entrypoint = "unknown"
	}
	command := host.Command

	result := map[string]any{
		"version":    version,
		"args":       args,
		"cwd":        cwd,
		"entrypoint": entrypoint,
		"command":    command,
		"solution":   solutionData(ctx, host),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"shell":      detectShell(),
	}

	lgr.V(1).Info("provider completed", "provider", ProviderName)
	return &sdkprovider.Output{Data: result}, nil
}

// effectiveHost returns the host metadata for this execution. It prefers the
// per-execution settings delivered via context (the SDK server merges
// execute-time over configure-time settings, so this is correct under both
// pool-mode and per-call hosts) and falls back to the ConfigureProvider copy
// when the context carries no settings (older hosts).
func (p *Plugin) effectiveHost(ctx context.Context) hostMetadata {
	settings, ok := sdkprovider.SettingsFromContext(ctx)
	if !ok {
		return p.host
	}
	raw, ok := settings["metadata"]
	if !ok {
		return p.host
	}
	var host hostMetadata
	if err := json.Unmarshal(raw, &host); err != nil {
		// Malformed per-execution settings: fall back to the stored copy.
		return p.host
	}
	return host
}

// solutionData builds the solution output map. It prefers the canonical
// per-execution solution metadata delivered via context
// (sdkprovider.SolutionMetadataFromContext), which is the fresh, authoritative
// source under pool-mode hosts. It falls back to the solution embedded in the
// effective host metadata when the accessor is absent (older hosts), and to an
// empty map when neither carries solution data.
func solutionData(ctx context.Context, host hostMetadata) map[string]any {
	if meta, ok := sdkprovider.SolutionMetadataFromContext(ctx); ok && meta != nil &&
		(meta.Name != "" || meta.Version != "") {
		return map[string]any{
			"name":        meta.Name,
			"version":     meta.Version,
			"displayName": meta.DisplayName,
			"description": meta.Description,
			"category":    meta.Category,
			"tags":        meta.Tags,
			"source":      meta.Source,
		}
	}

	sol := host.Solution
	if sol.Name != "" || sol.Version != "" {
		return map[string]any{
			"name":        sol.Name,
			"version":     sol.Version,
			"displayName": sol.DisplayName,
			"description": sol.Description,
			"category":    sol.Category,
			"tags":        sol.Tags,
			"source":      sol.Source,
		}
	}

	return map[string]any{}
}

// DescribeWhatIf returns a description of what the provider would do.
//
//nolint:revive // ctx required by interface
func (p *Plugin) DescribeWhatIf(_ context.Context, providerName string, _ map[string]any) (string, error) {
	if providerName != ProviderName {
		return "", fmt.Errorf("unknown provider: %s", providerName)
	}

	return "Would return runtime metadata (version, args, cwd, entrypoint, command, solution, os, arch, shell)", nil
}

// ExecuteProviderStream is not supported by the metadata provider.
//
//nolint:revive // all params required by interface
func (p *Plugin) ExecuteProviderStream(_ context.Context, _ string, _ map[string]any, _ func(sdkplugin.StreamChunk)) error {
	return sdkplugin.ErrStreamingNotSupported
}

// ExtractDependencies returns resolver keys this input depends on.
//
//nolint:revive // all params required by interface
func (p *Plugin) ExtractDependencies(_ context.Context, _ string, _ map[string]any) ([]string, error) {
	return nil, nil
}

// StopProvider performs cleanup for the named provider.
//
//nolint:revive // all params required by interface
func (p *Plugin) StopProvider(_ context.Context, _ string) error {
	return nil
}

// detectShell returns the base name of the user's shell.
//
// On Unix, $SHELL is the canonical source (user's configured login shell).
//
// On Windows, %ComSpec% always points to cmd.exe regardless of the running
// shell, so we use a multi-step heuristic:
//  1. $SHELL set -> authoritative on Unix; also covers Git Bash / MSYS2 / Cygwin on Windows.
//  2. PSModulePath set (Windows only) -> PowerShell session. Inspect the parent
//     process name to distinguish "pwsh" (PowerShell 7+) from "powershell"
//     (Windows PowerShell 5.x). Falls back to "pwsh" if introspection fails.
//  3. %ComSpec% -> last resort on Windows (almost always cmd.exe).
//
// Returns an empty string if nothing is detected.
func detectShell() string {
	// Unix fast path: $SHELL is authoritative.
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}

	// Windows heuristic: PSModulePath is set in every PowerShell session.
	if goosFunc() == "windows" {
		if os.Getenv("PSModulePath") != "" {
			return detectPowerShellVariant()
		}
	}

	// Fallback: %ComSpec% on Windows (cmd.exe), empty on Unix.
	if comspec := os.Getenv("ComSpec"); comspec != "" {
		return filepath.Base(comspec)
	}
	return ""
}

// detectPowerShellVariant inspects the parent process to distinguish
// "pwsh" (PowerShell 7+) from "powershell" (Windows PowerShell 5.x).
// Returns "pwsh" if the parent cannot be determined.
func detectPowerShellVariant() string {
	name := parentProcessNameFunc()
	if name == "" {
		return "pwsh"
	}

	base := filepath.Base(name)
	base = strings.TrimSuffix(base, ".exe")
	switch base {
	case "powershell":
		return "powershell"
	case "pwsh":
		return "pwsh"
	}

	// Default to modern PowerShell if parent is something unexpected.
	return "pwsh"
}
