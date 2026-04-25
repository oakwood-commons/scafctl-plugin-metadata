// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"context"
	"encoding/json"
	"testing"

	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"
)

func BenchmarkMetadataProvider_Execute(b *testing.B) {
	p := NewPlugin()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = p.ExecuteProvider(ctx, ProviderName, nil)
	}
}

func BenchmarkMetadataProvider_Execute_WithConfig(b *testing.B) {
	p := NewPlugin()

	meta := hostMetadata{
		BuildVersion: "1.2.3",
		Commit:       "abc123",
		BuildTime:    "2025-01-01T00:00:00Z",
		Entrypoint:   "cli",
		Command:      "scafctl/run/solution",
		Args:         []string{"scafctl", "run", "solution"},
		Solution: solutionMeta{
			Name:    "my-solution",
			Version: "1.0.0",
		},
	}
	raw, _ := json.Marshal(meta)
	_ = p.ConfigureProvider(context.Background(), ProviderName, sdkplugin.ProviderConfig{
		Settings: map[string]json.RawMessage{"metadata": raw},
	})

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = p.ExecuteProvider(ctx, ProviderName, nil)
	}
}

func BenchmarkMetadataProvider_DescribeWhatIf(b *testing.B) {
	p := NewPlugin()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = p.DescribeWhatIf(ctx, ProviderName, nil)
	}
}
