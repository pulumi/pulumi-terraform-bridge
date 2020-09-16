package provider

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/tfplugin5"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/plugins"
)

func collectTerraformNames(m shim.ResourceMap) []string {
	var names []string
	m.Range(func(name string, _ shim.Resource) bool {
		names = append(names, name)
		return true
	})
	sort.Strings(names)
	return names
}

func makePulumiToken(providerReference, memberName string, isDataSource bool) (string, bool) {
	// Extract the member name from the resource or data source name.
	underscore := strings.IndexRune(memberName, '_')
	if underscore == -1 {
		return "", false
	}
	member := memberName[underscore+1:]
	if len(member) == 0 {
		return "", false
	}

	// Convert the member name from snake_case to PascalCase
	member = tfbridge.TerraformToPulumiName(member, nil, nil, true)
	if isDataSource {
		member = "get" + member
	}

	// Build the token
	return fmt.Sprintf("tfx:%v:%v", providerReference, member), true
}

func makePulumiResources(providerReference string, m shim.ResourceMap) map[string]*tfbridge.ResourceInfo {
	schemas := map[string]*tfbridge.ResourceInfo{}
	for _, resourceTypeName := range collectTerraformNames(m) {
		token, ok := makePulumiToken(providerReference, resourceTypeName, false)
		if !ok {
			log.Printf("WARNING: malformed resource type name '%s'", resourceTypeName)
			continue
		}
		schemas[resourceTypeName] = &tfbridge.ResourceInfo{Tok: tokens.Type(token)}
	}
	return schemas
}

func makePulumiDataSources(providerReference string, m shim.ResourceMap) map[string]*tfbridge.DataSourceInfo {
	schemas := map[string]*tfbridge.DataSourceInfo{}
	for _, dataSourceName := range collectTerraformNames(m) {
		token, ok := makePulumiToken(providerReference, dataSourceName, true)
		if !ok {
			log.Printf("WARNING: malformed data source name '%s'", dataSourceName)
			continue
		}
		schemas[dataSourceName] = &tfbridge.DataSourceInfo{Tok: tokens.ModuleMember(token)}
	}
	return schemas
}

func GetProviderInfo(p shim.Provider, meta plugins.PluginMeta) tfbridge.ProviderInfo {
	providerReference := meta.String()
	return tfbridge.ProviderInfo{
		P:           p,
		Name:        meta.Name,
		Version:     meta.Version.String(),
		Resources:   makePulumiResources(providerReference, p.ResourcesMap()),
		DataSources: makePulumiDataSources(providerReference, p.DataSourcesMap()),
		JavaScript: &tfbridge.JavaScriptInfo{
			Dependencies: map[string]string{
				"@pulumi/pulumi": "^2.0.0",
			},
			DevDependencies: map[string]string{
				"@types/node": "^8.0.0", // so we can access strongly typed node definitions.
			},
		},
		Python: &tfbridge.PythonInfo{
			Requires: map[string]string{
				"pulumi": ">=2.9.0,<3.0.0",
			},
			UsesIOClasses: true,
		},
		CSharp: &tfbridge.CSharpInfo{
			PackageReferences: map[string]string{
				"Pulumi":                       "2.*",
				"System.Collections.Immutable": "1.6.0",
			},
		},
		PulumiSchema: &tfbridge.PulumiSchemaInfo{
			ModuleFormat: &tfbridge.ModuleFormatInfo{
				Format: "(?:.*)()",
				NewTypeToken: func(pkg, mod, name string) string {
					return fmt.Sprintf("tfx:%v:%v", mod, name)
				},
			},
		},
	}
}

func StartProvider(ctx context.Context, meta plugins.PluginMeta) (tfbridge.ProviderInfo, error) {
	p, err := tfplugin5.StartProvider(ctx, meta.ExecutablePath, "0.13.2")
	if err != nil {
		return tfbridge.ProviderInfo{}, err
	}
	return GetProviderInfo(p, meta), nil
}

func StartProviders(ctx context.Context, plugins ...plugins.PluginMeta) ([]tfbridge.ProviderInfo, error) {
	infos := make([]tfbridge.ProviderInfo, len(plugins))
	for i, pluginMeta := range plugins {
		info, err := StartProvider(ctx, pluginMeta)
		if err != nil {
			return nil, err
		}
		infos[i] = info
	}
	return infos, nil
}
