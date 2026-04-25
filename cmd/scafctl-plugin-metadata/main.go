// Package main is the entry point for the scafctl-plugin-metadata plugin.
package main

import (
	"github.com/oakwood-commons/scafctl-plugin-metadata/internal/metadata"

	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"
)

func main() {
	sdkplugin.Serve(metadata.NewPlugin())
}
