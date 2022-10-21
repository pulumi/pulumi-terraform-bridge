// Copyright 2016-2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package info

import (
	"fmt"
	"github.com/blang/semver"

	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type ProviderInfo struct {
	P              func() tfsdkprovider.Provider
	Name           string // the TF provider name (e.g. terraform-provider-XXXX).
	ResourcePrefix string // the prefix on resources the provider exposes, if different to `Name`.
	// GitHubOrg is the segment of the upstream provider's Go module path that comes after GitHubHost and before
	// terraform-provider-${Name}. Defaults to `terraform-providers`.
	//
	// Note that this value should match the require directive for the upstream provider, not any replace directives.
	//
	// For example, GitHubOrg should be set to "my-company" given the following go.mod:
	//
	// require github.com/my-company/terraform-repo-example v1.0.0
	// replace github.com/my-company/terraform-repo-example => github.com/some-fork/terraform-repo-example v1.0.0
	GitHubOrg   string   // the GitHub org of the provider. Defaults to `terraform-providers`.
	GitHubHost  string   // the GitHub host for the provider. Defaults to `github.com`.
	Description string   // an optional descriptive overview of the package (a default supplied).
	Keywords    []string // an optional list of keywords to help discovery of this package. e.g. "category/cloud, category/infrastructure"
	License     string   // the license, if any, the resulting package has (default is none).
	LogoURL     string   // an optional URL to the logo of the package
	DisplayName string   // the human friendly name of the package used in the Pulumi registry
	Publisher   string   // the name of the person or organization that authored and published the package.
	Homepage    string   // the URL to the project homepage.
	Repository  string   // the URL to the project source code repository.
	Version     string   // the version of the provider package.

	// Config      map[string]*SchemaInfo             // a map of TF name to config schema overrides.
	// ExtraConfig map[string]*ConfigInfo             // a list of Pulumi-only configuration variables.
	Resources map[string]*ResourceInfo // a map of TF name to Pulumi name; standard mangling occurs if no entry.
	// DataSources map[string]*DataSourceInfo         // a map of TF name to Pulumi resource info.
	// ExtraTypes  map[string]pschema.ComplexTypeSpec // a map of Pulumi token to schema type for overlaid types.

	// ExtraResourceHclExamples is a slice of additional HCL examples attached to resources which are converted to the
	// relevant target language(s)
	// ExtraResourceHclExamples []HclExampler

	// ExtraFunctionHclExamples is a slice of additional HCL examples attached to functions which are converted to the
	// relevant target language(s)

	// ExtraFunctionHclExamples []HclExampler
	IgnoreMappings    []string        // a list of TF resources and data sources to ignore in mappings errors
	PluginDownloadURL string          // an optional URL to download the provider binary from.
	JavaScript        *JavaScriptInfo // optional overlay information for augmented JavaScript code-generation.
	Python            *PythonInfo     // optional overlay information for augmented Python code-generation.
	Golang            *GolangInfo     // optional overlay information for augmented Golang code-generation.
	CSharp            *CSharpInfo     // optional overlay information for augmented C# code-generation.
	Java              *JavaInfo       // optional overlay information for augmented C# code-generation.
	TFProviderVersion string          // the version of the TF provider on which this was based
	// TFProviderLicense        *TFProviderLicense // license that the TF provider is distributed under. Default `MPL 2.0`.
	TFProviderModuleVersion string // the Go module version of the provider. Default is unversioned e.g. v1

	// PreConfigureCallback           PreConfigureCallback // a provider-specific callback to invoke prior to TF Configure
	// PreConfigureCallbackWithLogger PreConfigureCallbackWithLogger
}

// OverlayInfo contains optional overlay information.  Each info has a 1:1 correspondence with a module and
// permits extra files to be included from the overlays/ directory when building up packs/.  This allows augmented
// code-generation for convenient things like helper functions, modules, and gradual typing.
type OverlayInfo struct {
	DestFiles []string                // Additional files to include in the index file. Must exist in the destination.
	Modules   map[string]*OverlayInfo // extra modules to inject into the structure.
}

// JavaScriptInfo contains optional overlay information for Python code-generation.
type JavaScriptInfo struct {
	PackageName       string            // Custom name for the NPM package.
	Dependencies      map[string]string // NPM dependencies to add to package.json.
	DevDependencies   map[string]string // NPM dev-dependencies to add to package.json.
	PeerDependencies  map[string]string // NPM peer-dependencies to add to package.json.
	Resolutions       map[string]string // NPM resolutions to add to package.json.
	Overlay           *OverlayInfo      // optional overlay information for augmented code-generation.
	TypeScriptVersion string            // A specific version of TypeScript to include in package.json.
}

// PythonInfo contains optional overlay information for Python code-generation.
type PythonInfo struct {
	Requires      map[string]string // Pip install_requires information.
	Overlay       *OverlayInfo      // optional overlay information for augmented code-generation.
	UsesIOClasses bool              // Deprecated: No longer required, all providers use IO classes.
	PackageName   string            // Name of the Python package to generate
}

// GolangInfo contains optional overlay information for Golang code-generation.
type GolangInfo struct {
	GenerateResourceContainerTypes bool         // Generate container types for resources e.g. arrays, maps, pointers etc.
	ImportBasePath                 string       // Base import path for package.
	Overlay                        *OverlayInfo // optional overlay information for augmented code-generation.
}

// CSharpInfo contains optional overlay information for C# code-generation.
type CSharpInfo struct {
	PackageReferences map[string]string // NuGet package reference information.
	Overlay           *OverlayInfo      // optional overlay information for augmented code-generation.
	Namespaces        map[string]string // Known .NET namespaces with proper capitalization.
	RootNamespace     string            // The root namespace if setting to something other than Pulumi in the package name
}

type JavaInfo struct {
	BasePackage string // the Base package for the Java SDK
}

// ResourceInfo is a top-level type exported by a provider.  This structure can override the type to generate.  It can
// also give custom metadata for fields, using the SchemaInfo structure below.  Finally, a set of composite keys can be
// given; this is used when Terraform needs more than just the ID to uniquely identify and query for a resource.
type ResourceInfo struct {
	Tok      tokens.Type            // a type token to override the default; "" uses the default.
	Fields   map[string]*SchemaInfo // a map of custom field names; if a type is missing, uses the default.
	IDFields []string               // an optional list of ID alias fields.
	// list of parameters that we can trust that any change will allow a createBeforeDelete
	UniqueNameFields []string
	//Docs                *DocInfo    // overrides for finding and mapping TF docs.
	DeleteBeforeReplace bool // if true, Pulumi will delete before creating new replacement resources.
	//Aliases             []AliasInfo // aliases for this resources, if any.
	DeprecationMessage string // message to use in deprecation warning
	CSharpName         string // .NET-specific name

}

// GetTok returns a resource type token
func (info *ResourceInfo) GetTok() tokens.Token { return tokens.Token(info.Tok) }

// GetFields returns information about the resource's custom fields
func (info *ResourceInfo) GetFields() map[string]*SchemaInfo { return info.Fields }

// GetDocs returns a resource docs override from the Pulumi provider
// func (info *ResourceInfo) GetDocs() *DocInfo { return info.Docs }

// ReplaceExamplesSection returns whether to replace the upstream examples with our own source
// func (info *ResourceInfo) ReplaceExamplesSection() bool {
//	return info.Docs != nil && info.Docs.ReplaceExamplesSection
//}

// SchemaInfo contains optional name transformations to apply.
type SchemaInfo struct {
	// a name to override the default; "" uses the default.
	Name string

	// a name to override the default when targeting C#; "" uses the default.
	CSharpName string

	// a type to override the default; "" uses the default.
	Type tokens.Type

	// alternative types that can be used instead of the override.
	AltTypes []tokens.Type

	// a type to override when the property is a nested structure.
	NestedType tokens.Type

	// an optional idemponent transformation, applied before passing to TF.
	// Transform Transformer

	// a schema override for elements for arrays, maps, and sets.
	Elem *SchemaInfo

	// a map of custom field names; if a type is missing, the default is used.
	Fields map[string]*SchemaInfo

	// a map of asset translation information, if this is an asset.
	// Asset *AssetTranslation

	// an optional default directive to be applied if a value is missing.
	// Default *DefaultInfo

	// to override whether a property is stable or not.
	Stable *bool

	// to override whether this property should project as a scalar or array.
	MaxItemsOne *bool

	// to remove empty object array elements
	SuppressEmptyMapElements *bool

	// this will make the parameter as computed and not allow the user to set it
	MarkAsComputedOnly *bool

	// this will make the parameter optional in the schema
	MarkAsOptional *bool

	// the deprecation message for the property
	DeprecationMessage string

	// whether a change in the configuration would force a new resource
	ForceNew *bool

	// whether or not this property has been removed from the Terraform schema
	Removed bool

	// if set, this property will not be added to the schema and no bindings will be generated for it
	Omit bool

	// whether or not to treat this property as secret
	Secret *bool
}

// Calculates the major version of a go sdk
// go module paths only care about appending a version when the version is
// 2 or greater. github.com/org/my-repo/sdk/v1/go is not a valid
// go module path but github.com/org/my-repo/sdk/v2/go is
func GetModuleMajorVersion(version string) string {
	var majorVersion string
	sver, err := semver.ParseTolerant(version)
	if err != nil {
		panic(err)
	}
	if sver.Major > 1 {
		majorVersion = fmt.Sprintf("v%d", sver.Major)
	}
	return majorVersion
}
