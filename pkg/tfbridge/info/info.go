// Copyright 2016-2024, Pulumi Corporation.
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

// [info] contains the types that bridged provider authors use to describe the mapping
// between Pulumi and Terraform providers.
//
// As much as possible, this package should restrict itself to type declarations. Runtime
// behavior and advanced configuration should go in /pkg/tfbridge or more specialized
// packages.
package info

import (
	"context"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

const (
	MPL20LicenseType      TFProviderLicense = "MPL 2.0"
	MITLicenseType        TFProviderLicense = "MIT"
	Apache20LicenseType   TFProviderLicense = "Apache 2.0"
	UnlicensedLicenseType TFProviderLicense = "UNLICENSED"
)

// Provider contains information about a Terraform provider plugin that we will use to
// generate the Pulumi metadata.  It primarily contains a pointer to the Terraform schema,
// but can also contain specific name translations.
//
//nolint:lll
type Provider struct {
	P              shim.Provider // the TF provider/schema.
	Name           string        // the TF provider name (e.g. terraform-provider-XXXX).
	ResourcePrefix string        // the prefix on resources the provider exposes, if different to `Name`.
	// GitHubOrg is the segment of the upstream provider's Go module path that comes after GitHubHost and before
	// terraform-provider-${Name}. Defaults to `terraform-providers`.
	//
	// Note that this value should match the require directive for the upstream provider, not any replace directives.
	//
	// For example, GitHubOrg should be set to "my-company" given the following go.mod:
	//
	// require github.com/my-company/terraform-repo-example v1.0.0
	// replace github.com/my-company/terraform-repo-example => github.com/some-fork/terraform-repo-example v1.0.0
	GitHubOrg      string                             // the GitHub org of the provider. Defaults to `terraform-providers`.
	GitHubHost     string                             // the GitHub host for the provider. Defaults to `github.com`.
	Description    string                             // an optional descriptive overview of the package (a default supplied).
	Keywords       []string                           // an optional list of keywords to help discovery of this package. e.g. "category/cloud, category/infrastructure"
	License        string                             // the license, if any, the resulting package has (default is none).
	LogoURL        string                             // an optional URL to the logo of the package
	DisplayName    string                             // the human friendly name of the package used in the Pulumi registry
	Publisher      string                             // the name of the person or organization that authored and published the package.
	Homepage       string                             // the URL to the project homepage.
	Repository     string                             // the URL to the project source code repository.
	Version        string                             // the version of the provider package.
	Config         map[string]*Schema                 // a map of TF name to config schema overrides.
	ExtraConfig    map[string]*Config                 // a list of Pulumi-only configuration variables.
	Resources      map[string]*Resource               // a map of TF type or renamed entity name to Pulumi resource info.
	DataSources    map[string]*DataSource             // a map of TF type or renamed entity name to Pulumi resource info.
	Actions        map[string]*Action                 // a map of TF type or renamed entity name to Pulumi action info.
	ExtraTypes     map[string]pschema.ComplexTypeSpec // a map of Pulumi token to schema type for extra types.
	ExtraResources map[string]pschema.ResourceSpec    // a map of Pulumi token to schema type for extra resources.
	ExtraFunctions map[string]pschema.FunctionSpec    // a map of Pulumi token to schema type for extra functions.

	// ExtraResourceHclExamples is a slice of additional HCL examples attached to resources which are converted to the
	// relevant target language(s)
	ExtraResourceHclExamples []HclExampler
	// ExtraFunctionHclExamples is a slice of additional HCL examples attached to functions which are converted to the
	// relevant target language(s)
	ExtraFunctionHclExamples []HclExampler
	// IgnoreMappings is a list of TF resources and data sources that are known to be unmapped.
	//
	// These resources/data sources do not generate missing mappings errors and will not be automatically
	// mapped.
	//
	// If there is a mapping in Resources or DataSources, it can override IgnoreMappings. This is common
	// when you need to ignore a datasource but not the resource with the same name, or vice versa.
	IgnoreMappings          []string
	PluginDownloadURL       string             // an optional URL to download the provider binary from.
	JavaScript              *JavaScript        // optional overlay information for augmented JavaScript code-generation.
	Python                  *Python            // optional overlay information for augmented Python code-generation.
	Golang                  *Golang            // optional overlay information for augmented Golang code-generation.
	CSharp                  *CSharp            // optional overlay information for augmented C# code-generation.
	Java                    *Java              // optional overlay information for augmented Java code-generation.
	TFProviderVersion       string             // the version of the TF provider on which this was based
	TFProviderLicense       *TFProviderLicense // license that the TF provider is distributed under. Default `MPL 2.0`.
	TFProviderModuleVersion string             // the Go module version of the provider. Default is unversioned e.g. v1

	// a provider-specific callback to invoke prior to TF Configure
	// Any CheckFailureErrors returned from PreConfigureCallback are converted to
	// CheckFailures and returned as failures instead of errors in CheckConfig
	PreConfigureCallback PreConfigureCallback
	// Any CheckFailureErrors returned from PreConfigureCallbackWithLogger are
	// converted to CheckFailures and returned as failures instead of errors in CheckConfig
	PreConfigureCallbackWithLogger PreConfigureCallbackWithLogger

	// Information for the embedded metadata file.
	//
	// See NewProviderMetadata for in-place construction of a *MetadataInfo.
	// If a provider should be mixed in with the Terraform provider with MuxWith (see below)
	// this field must be initialized.
	MetadataInfo *Metadata

	// Rules that control file discovery and edits for any subset of docs in a provider.
	DocRules *DocRule

	// An optional local file path to the root of the upstream provider's git repo, for use in docs generation.
	//
	// If UpstreamRepoPath is left blank, it is inferred to the location where Go downloaded the build
	// dependency of the provider. The following fields influence the inference decision:
	//
	// - GitHubHost
	// - GitHubOrg
	// - Name
	// - TFProviderModuleVersion
	//
	// This list may change over time.
	UpstreamRepoPath string

	// EXPERIMENTAL: the signature may change in minor releases.
	//
	// Optional function to post-process the generated schema spec after
	// the bridge completed its original version based on the TF schema.
	// A hook to enable custom schema modifications specific to a provider.
	SchemaPostProcessor func(spec *pschema.PackageSpec)

	// The MuxWith array allows the mixin (muxing) of other providers to the wrapped upstream Terraform provider.
	// With a provider mixin it's possible to add or replace resources and/or functions (data sources) in the wrapped
	// Terraform provider without having to change the upstream code itself. If multiple provider mixins are specified
	// the schema generator in pkg/tfgen will call the GetSpec() method of muxer.Provider in sequence. Thus, if more or two
	// of the mixins define the same resource/function, the last definition will end up in the combined schema of the
	// compiled provider.
	MuxWith []MuxProvider

	// Disables validation of provider-level configuration for Plugin Framework based providers.
	// Hybrid providers that utilize a mixture of Plugin Framework and SDKv2 based resources may
	// opt into this to workaround slowdown in PF validators, since their configuration is
	// already being checked by SDKv2 based validators.
	//
	// See also: pulumi/pulumi-terraform-bridge#1448
	SkipValidateProviderConfigForPluginFramework bool

	// Enables generation of a trimmed, runtime-only metadata file
	// to help reduce resource plugin start time
	//
	// See also pulumi/pulumi-terraform-bridge#1524
	GenerateRuntimeMetadata bool
	// EnableZeroDefaultSchemaVersion makes the provider default
	// to version 0 when no version is specified in the state of a resource.
	EnableZeroDefaultSchemaVersion bool
	// Deprecated: This flag is enabled by default and will be removed in a future release.
	// EnableAccurateBridgePreview makes the SDKv2 bridge use an experimental feature
	// to generate more accurate diffs and previews for resources
	EnableAccurateBridgePreview bool
	// EnableAccuratePFBridgePreview makes the Plugin Framework bridge use an experimental feature
	// to generate more accurate diffs and previews for resources
	EnableAccuratePFBridgePreview bool

	// Deprecated: This flag is enabled by default and will be removed in a future release.
	// Newer versions of the bridge preserve Terraform raw state by saving the delta between Pulumi state and
	// Terraform raw state into the state file. Setting this to true enables the feature.
	EnableRawStateDelta bool

	// DisableRequiredWithDefaultTurningOptional disables making required properties optional if they have a default value.
	DisableRequiredWithDefaultTurningOptional bool

	// Check generated schema for dangling references. Newer providers should opt into this.
	NoDanglingReferences bool
}

// HclExampler represents a supplemental HCL example for a given resource or function.
type HclExampler interface {
	// GetToken returns the fully qualified path to the resource or function in the schema, e.g.
	// "provider:module/getFoo:getFoo" (function), or
	// "provider:module/bar:Bar" (resource)
	GetToken() string
	// GetMarkdown returns the Markdown that comprises the entire example, including the header.
	//
	// Headers should be an H3 ("###") and the header content should not contain any prefix, e.g. "Foo with Bar" not,
	// "Example Usage - Foo with Bar".
	//
	// Code should be surrounded with code fences with an indicator of the language on the opening fence, e.g. "```hcl".
	GetMarkdown() (string, error)
}

func (info *Provider) GetConfig() map[string]*Schema {
	if info.Config != nil {
		return info.Config
	}
	return map[string]*Schema{}
}

// The function used to produce the set of edit rules for a provider.
//
// For example, if you want to skip default edits, you would use the function:
//
//	func([]DocsEdit) []DocsEdit { return nil }
//
// If you wanted to incorporate custom edits, default edits, and then a check that the
// resulting document is valid, you would use the function:
//
//	func(defaults []DocsEdit) []DocsEdit {
//		return append(customEdits, append(defaults, validityCheck)...)
//	}
type MakeEditRules func(defaults []DocsEdit) []DocsEdit

// DocRule controls file discovery and edits for any subset of docs in a provider.
type DocRule struct {
	// The function called to get the set of edit rules to use.
	//
	// defaults represents suggested edit rules. If EditRules is `nil`, defaults is
	// used as is.
	EditRules MakeEditRules

	// A function to suggest alternative file names for a TF element.
	//
	// When the bridge loads the documentation for a resource or a datasource, it
	// infers the name of the file that contains the documentation. AlternativeNames
	// allows you to provide a provider specific extension to the override list.
	//
	// For example, when attempting to find the documentation for the resource token
	// aws_waf_instances, the bridge will check the following files (in order):
	//
	//	"waf_instance.html.markdown"
	//	"waf_instance.markdown"
	//	"waf_instance.html.md"
	//	"waf_instance.md"
	//	"aws_waf_instance.html.markdown"
	//	"aws_waf_instance.markdown"
	//	"aws_waf_instance.html.md"
	//	"aws_waf_instance.md"
	//
	// The bridge will check any file names returned by AlternativeNames before
	// checking it's standard list.
	AlternativeNames func(DocsPath) []string
}

// Information for file lookup.
type DocsPath struct {
	TfToken string
}

type EditPhase int

const (
	// PreCodeTranslation directs an info.DocsEdit to occur before resource example code is translated.
	PreCodeTranslation EditPhase = iota
	// PostCodeTranslation directs an info.DocsEdit to occur after resource example code is translated.
	// It should be used when a docs edit would otherwise affect the code conversion mechanics.
	// TODO[https://github.com/pulumi/pulumi-terraform-bridge/issues/2459]: Right now, PostCodeTranslation is only
	// called on installation docs.
	PostCodeTranslation
)

type DocsEdit struct {
	// The file name at which this rule applies. File names are matched via filepath.Match.
	//
	// To match all files, supply "*".
	//
	// All 4 of these names will match "waf_instances.html.markdown":
	//
	// - "waf_instances.html.markdown"
	// - "waf_instances.*"
	// - "waf*"
	// - "*"
	//
	// Provider resources are sourced directly from the TF schema, and as such have an
	// empty path.
	Path string
	// The function that performs the edit on the file bytes.
	//
	// Must not be nil.
	Edit func(path string, content []byte) ([]byte, error)
	// Phase determines when the edit rule will run.
	//
	// The default phase is [PreCodeTranslation].
	Phase EditPhase
}

// TFProviderLicense is a way to be able to pass a license type for the upstream Terraform provider.
type TFProviderLicense string

// GetResourcePrefix returns the resource prefix for the provider: info.ResourcePrefix
// if that is set, or info.Name if not. This is to avoid unexpected behavior with providers
// which have no need to set ResourcePrefix following its introduction.
func (info Provider) GetResourcePrefix() string {
	if info.ResourcePrefix == "" {
		return info.Name
	}

	return info.ResourcePrefix
}

func (info Provider) GetMetadata() ProviderMetadata {
	info.MetadataInfo.assertValid()
	return info.MetadataInfo.Data
}

func (info Provider) GetGitHubOrg() string {
	if info.GitHubOrg == "" {
		return "terraform-providers"
	}

	return info.GitHubOrg
}

func (info Provider) GetGitHubHost() string {
	if info.GitHubHost == "" {
		return "github.com"
	}

	return info.GitHubHost
}

func (info Provider) GetTFProviderLicense() TFProviderLicense {
	if info.TFProviderLicense == nil {
		return MPL20LicenseType
	}

	return *info.TFProviderLicense
}

func (info Provider) GetProviderModuleVersion() string {
	if info.TFProviderModuleVersion == "" {
		return "" // there is no such thing as a v1 module - there is just a missing version declaration
	}

	return info.TFProviderModuleVersion
}

// Alias is a partial description of prior named used for a resource. It can be processed in the
// context of a resource creation to determine what the full aliased URN would be.
//
// It can be used by Pulumi resource providers to change the aspects of it (i.e. what module it is
// contained in), without causing resources to be recreated for customers who migrate from the
// original resource to the current resource.
type Alias struct {
	Type *string

	// Deprecated: name aliases are not supported and will be removed.
	Name *string

	// Deprecated: project aliases are not supported and will be removed.
	Project *string
}

// ResourceOrDataSource is a shared interface to ResourceInfo and DataSourceInfo mappings
type ResourceOrDataSource interface {
	GetTok() tokens.Token          // a type token to override the default; "" uses the default.
	GetFields() map[string]*Schema // a map of custom field names; if a type is missing, uses the default.
	GetDocs() *Doc                 // overrides for finding and mapping TF docs.
	ReplaceExamplesSection() bool  // whether we are replacing the upstream TF examples generation
}

// Resource is a top-level type exported by a provider.  This structure can override the type to generate.  It can
// also give custom metadata for fields, using the SchemaInfo structure below.  Finally, a set of composite keys can be
// given; this is used when Terraform needs more than just the ID to uniquely identify and query for a resource.
type Resource struct {
	Tok    tokens.Type        // a type token to override the default; "" uses the default.
	Fields map[string]*Schema // a map of custom field names; if a type is missing, uses the default.

	// Deprecated: IDFields is not currently used and will be removed in the next major version of
	// pulumi-terraform-bridge. See [ComputeID].
	IDFields []string

	// list of parameters that we can trust that any change will allow a createBeforeDelete
	UniqueNameFields    []string
	Docs                *Doc    // overrides for finding and mapping TF docs.
	DeleteBeforeReplace bool    // if true, Pulumi will delete before creating new replacement resources.
	Aliases             []Alias // aliases for this resources, if any.
	DeprecationMessage  string  // message to use in deprecation warning
	CSharpName          string  // .NET-specific name

	// Optional hook to run before upgrading the state. TODO[pulumi/pulumi-terraform-bridge#864] this is currently
	// only supported for Plugin-Framework based providers.
	PreStateUpgradeHook PreStateUpgradeHook

	// An experimental way to augment the Check function in the Pulumi life cycle.
	PreCheckCallback PreCheckCallback

	// Resource operations such as Create, Read, and Update return the resource outputs to be
	// recored in Pulumi statefile. TransformOutputs provides the last chance to edit these
	// outputs before they are stored. In particular, it can be used as a last resort hook to
	// make corrections in the default translation of the resource state from TF to Pulumi.
	// Should be used sparingly.
	TransformOutputs PropertyTransform

	// Check, Diff, Read, Update and Delete refer to old inputs sourced from the
	// Pulumi statefile. TransformFromState lets providers edit these outputs before they
	// are accessed by other provider functions or by terraform. In particular, it can
	// be used to perform upgrades on old pulumi state.  Should be used sparingly.
	TransformFromState PropertyTransform

	// Customizes inferring resource identity from state.
	//
	// The vast majority of resources define an "id" field that is recognized as the resource
	// identity. This is the default behavior when ComputeID is nil. There are some exceptions,
	// however, such as the RandomBytes resource, that base identity on a different field
	// ("base64" in the case of RandomBytes). ComputeID customization option supports such
	// resources. It is called in Create(preview=false) and Read provider methods.
	//
	// This option is currently only supported for Plugin Framework based resources.
	//
	// To delegate the resource ID to another string field in state, use the helper function
	// [DelegateIDField].
	ComputeID ComputeID
}

type ComputeID = func(ctx context.Context, state resource.PropertyMap) (resource.ID, error)

type PropertyTransform = func(context.Context, resource.PropertyMap) (resource.PropertyMap, error)

type PreCheckCallback = func(
	ctx context.Context, config resource.PropertyMap, meta resource.PropertyMap,
) (resource.PropertyMap, error)

// GetTok returns a resource type token
func (info *Resource) GetTok() tokens.Token { return tokens.Token(info.Tok) }

// GetFields returns information about the resource's custom fields
func (info *Resource) GetFields() map[string]*Schema {
	if info == nil {
		return nil
	}
	return info.Fields
}

// GetDocs returns a resource docs override from the Pulumi provider
func (info *Resource) GetDocs() *Doc { return info.Docs }

// ReplaceExamplesSection returns whether to replace the upstream examples with our own source
func (info *Resource) ReplaceExamplesSection() bool {
	return info.Docs != nil && info.Docs.ReplaceExamplesSection
}

// Action can be used to override an action's standard name mangling and argument information.
type Action struct {
	Tok                tokens.ModuleMember
	Fields             map[string]*Schema
	Docs               *Doc   // overrides for finding and mapping TF docs.
	DeprecationMessage string // message to use in deprecation warning
}

// GetTok returns a action type token
func (info *Action) GetTok() tokens.Token { return tokens.Token(info.Tok) }

// GetFields returns information about the action's custom fields
func (info *Action) GetFields() map[string]*Schema {
	if info == nil {
		return nil
	}
	return info.Fields
}

// GetDocs returns a action docs override from the Pulumi provider
func (info *Action) GetDocs() *Doc { return info.Docs }

// ReplaceExamplesSection returns whether to replace the upstream examples with our own source
func (info *Action) ReplaceExamplesSection() bool {
	return info.Docs != nil && info.Docs.ReplaceExamplesSection
}

// DataSource can be used to override a data source's standard name mangling and argument/return information.
type DataSource struct {
	Tok                tokens.ModuleMember
	Fields             map[string]*Schema
	Docs               *Doc   // overrides for finding and mapping TF docs.
	DeprecationMessage string // message to use in deprecation warning
}

// GetTok returns a datasource type token
func (info *DataSource) GetTok() tokens.Token { return tokens.Token(info.Tok) }

// GetFields returns information about the datasource's custom fields
func (info *DataSource) GetFields() map[string]*Schema {
	if info == nil {
		return nil
	}
	return info.Fields
}

// GetDocs returns a datasource docs override from the Pulumi provider
func (info *DataSource) GetDocs() *Doc { return info.Docs }

// ReplaceExamplesSection returns whether to replace the upstream examples with our own source
func (info *DataSource) ReplaceExamplesSection() bool {
	return info.Docs != nil && info.Docs.ReplaceExamplesSection
}

// Schema contains optional name transformations to apply.
type Schema struct {
	// a name to override the default; "" uses the default.
	Name string

	// a name to override the default when targeting C#; "" uses the default.
	CSharpName string

	// An optional Pulumi type token to use for the Pulumi type projection of the current property. When unset, the
	// default behavior is to generate fresh named Pulumi types as needed to represent the schema. To force the use
	// of a known type and avoid generating unnecessary types, use both [Type] and [OmitType].
	Type tokens.Type

	// Used together with [Type] to omit generating any Pulumi types whatsoever for the current property, and
	// instead use the object type identified by the token setup in [Type].
	//
	// It is an error to set [OmitType] to true without specifying [Type].
	//
	// Experimental.
	OmitType bool

	// alternative types that can be used instead of the override.
	AltTypes []tokens.Type

	// a type to override when the property is a nested structure.
	NestedType tokens.Type

	// an optional idemponent transformation, applied before passing to TF.
	Transform Transformer

	// a schema override for elements for arrays, maps, and sets.
	Elem *Schema

	// a map of custom field names; if a type is missing, the default is used.
	Fields map[string]*Schema

	// a map of asset translation information, if this is an asset.
	Asset *AssetTranslation

	// an optional default directive to be applied if a value is missing.
	Default *Default

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

	// Controls whether a change in the provider configuration should trigger a provider
	// replacement. While there is no matching concept in TF, Pulumi supports replacing explicit
	// providers and cascading the replacement to all resources provisioned with the given
	// provider configuration.
	//
	// This property is only relevant for [Provider.Config] properties.
	ForcesProviderReplace *bool

	// whether or not this property has been removed from the Terraform schema
	Removed bool

	// if set, this property will not be added to the schema and no bindings will be generated for it
	Omit bool

	// whether or not to treat this property as secret
	Secret *bool

	// Specifies the exact name to use for the generated type.
	//
	// When generating types for properties, by default Pulumi picks reasonable names based on the property path
	// prefix and the name of the property. Use [TypeName] to override this decision when the default names for
	// nested properties are too long or otherwise undesirable. The choice will further affect the automatically
	// generated names for any properties nested under the current one.
	//
	// Example use:
	//
	//     TypeName: tfbridge.Ref("Visual")
	//
	// Note that the type name, and not the full token like "aws:quicksight/Visual:Visual" is specified. The token
	// will be picked based on the current module ("quicksight" in the above example) where the parent resource or
	// data source is found.
	//
	// Experimental.
	TypeName *string

	// XAlwaysIncludeInImport prevents the field from pruning zero values when generating import inputs.
	//
	// This is necessary to accommodate Terraform providers that distinguish between the zero value of a property
	// and no value of the property.
	//
	// For example, consider cloudflare_ruleset resource (https://github.com/pulumi/pulumi-cloudflare/issues/957):
	//
	//	resources:
	//	  cache_rules:
	//	    type: cloudflare:Ruleset
	//	    properties:
	//	      kind: zone
	//	      rules:
	//	      - action: set_cache_settings
	//	        action_parameters:
	//	          cache: false
	//
	// Importing cache_rules will result in the following code:
	//
	//	resources:
	//	  cache_rules:
	//	    type: cloudflare:Ruleset
	//	    properties:
	//	      kind: zone
	//	      rules:
	//	      - action: set_cache_settings
	//
	// action_parameters.cache is dropped because "false" is the default value of boolean. Unfortunately, the
	// cloudflare provider treats "cache: false" very differently from "cache: null". Setting XAlwaysIncludeInImport
	// ensures that "cache: false" is always included in the resulting output.
	//
	// Experimental.
	XAlwaysIncludeInImport bool
}

// Config represents a synthetic configuration variable that is Pulumi-only, and not passed to Terraform.
type Config struct {
	// Info is the Pulumi schema for this variable.
	Info *Schema
	// Schema is the Terraform schema for this variable.
	Schema shim.Schema
}

// Transformer is given the option to transform a value in situ before it is processed by the bridge. This
// transformation must be deterministic and idempotent, and any value produced by this transformation must
// be a legal alternative input value. A good example is a resource that accepts either a string or
// JSON-stringable map; a resource provider may opt to store the raw string, but let users pass in maps as
// a convenience mechanism, and have the transformer stringify them on the fly. This is safe to do because
// the raw string is still accepted as a possible input value.
type Transformer func(resource.PropertyValue) (resource.PropertyValue, error)

// Doc contains optional overrides for finding and mapping TF docs.
type Doc struct {
	Source                         string // an optional override to locate TF docs; "" uses the default.
	Markdown                       []byte // an optional override for the source markdown.
	IncludeAttributesFrom          string // optionally include attributes from another raw resource for docs.
	IncludeArgumentsFrom           string // optionally include arguments from another raw resource for docs.
	IncludeAttributesFromArguments string // optionally include attributes from another raw resource's arguments.
	ImportDetails                  string // Overwrite for import instructions

	// Replace examples with the contents of a specific document
	// this document will satisfy the criteria `docs/pulumiToken.md`
	// The examples need to wrapped in the correct shortcodes
	ReplaceExamplesSection bool

	// Don't error when this doc is missing.
	//
	// This applies when PULUMI_MISSING_DOCS_ERROR="true".
	AllowMissing bool
}

// GetImportDetails returns a string of import instructions defined in the Pulumi provider. Defaults to empty.
func (info *Doc) GetImportDetails() string { return info.ImportDetails }

// HasDefault returns true if there is a default value for this property.
func (info Schema) HasDefault() bool {
	return info.Default != nil
}

// Default lets fields get default values at runtime, before they are even passed to Terraform.
type Default struct {
	// AutoNamed is true if this default represents an autogenerated name.
	AutoNamed bool
	// Config uses a configuration variable from this package as the default value.
	Config string

	// Deprecated. Use [Default.ComputeDefault].
	From func(res *PulumiResource) (interface{}, error)

	// ComputeDefault specifies a strategy for how to pick a default value when the user has not specified any value
	// in their program.
	//
	// One common use case for this functionality is auto-naming, see [AutoName] for a recommended starting point
	// and [ComputeAutoNameDefault] for the specific implementation.
	ComputeDefault func(ctx context.Context, opts ComputeDefaultOptions) (interface{}, error)

	// Value injects a raw literal value as the default.
	// Note that only simple types such as string, int and boolean are currently supported here.
	// Structs, slices and maps are not yet supported.
	Value interface{}
	// EnvVars to use for defaults. If none of these variables have values at runtime, the value of `Value` (if any)
	// will be used as the default.
	EnvVars []string
}

// ComputeDefaultAutonamingOptionsMode is the mode that controls how the provider handles the proposed name. If not
// specified, defaults to `Propose`.
type ComputeDefaultAutonamingOptionsMode int32

const (
	// ComputeDefaultAutonamingModePropose means the provider may use the proposed name as a suggestion but is free
	// to modify it.
	ComputeDefaultAutonamingModePropose ComputeDefaultAutonamingOptionsMode = iota
	// ComputeDefaultAutonamingModeEnforce means the provider must use exactly the proposed name (if present)
	// or return an error if the proposed name is invalid.
	ComputeDefaultAutonamingModeEnforce ComputeDefaultAutonamingOptionsMode = 1
	// ComputeDefaultAutonamingModeDisable means the provider should disable automatic naming and return an error
	// if no explicit name is provided by user's program.
	ComputeDefaultAutonamingModeDisable ComputeDefaultAutonamingOptionsMode = 2
)

// ComputeDefaultAutonamingOptions controls how auto-naming behaves when the engine provides explicit naming
// preferences. This is used by the engine to pass user preference for naming patterns.
type ComputeDefaultAutonamingOptions struct {
	ProposedName string
	Mode         ComputeDefaultAutonamingOptionsMode
}

// Configures [Default.ComputeDefault].
type ComputeDefaultOptions struct {
	// URN identifying the Resource. Set when computing default properties for a Resource, and unset for functions.
	URN resource.URN

	// Property map before computing the defaults.
	Properties resource.PropertyMap

	// Property map representing prior state, only set for non-Create Resource operations.
	PriorState resource.PropertyMap

	// PriorValue represents the last value of the current property in PriorState. It will have zero value if there
	// is no PriorState or if the property did not have a value in PriorState.
	PriorValue resource.PropertyValue

	// The engine provides a stable seed useful for generating random values consistently. This guarantees, for
	// example, that random values generated across "pulumi preview" and "pulumi up" in the same deployment are
	// consistent. This currently is only available for resource changes.
	Seed []byte

	// The engine can provide auto-naming options if the user configured an explicit preference for it.
	Autonaming *ComputeDefaultAutonamingOptions
}

// PulumiResource is just a little bundle that carries URN, seed and properties around.
type PulumiResource struct {
	URN        resource.URN
	Properties resource.PropertyMap
	Seed       []byte
	Autonaming *ComputeDefaultAutonamingOptions
}

// Overlay contains optional overlay information.  Each info has a 1:1 correspondence with a module and
// permits extra files to be included from the overlays/ directory when building up packs/.  This allows augmented
// code-generation for convenient things like helper functions, modules, and gradual typing.
type Overlay struct {
	DestFiles []string            // Additional files to include in the index file. Must exist in the destination.
	Modules   map[string]*Overlay // extra modules to inject into the structure.
}

// JavaScript contains optional overlay information for Python code-generation.
type JavaScript struct {
	PackageName       string            // Custom name for the NPM package.
	Dependencies      map[string]string // NPM dependencies to add to package.json.
	DevDependencies   map[string]string // NPM dev-dependencies to add to package.json.
	PeerDependencies  map[string]string // NPM peer-dependencies to add to package.json.
	Resolutions       map[string]string // NPM resolutions to add to package.json.
	Overlay           *Overlay          // optional overlay information for augmented code-generation.
	TypeScriptVersion string            // A specific version of TypeScript to include in package.json.
	PluginName        string            // The name of the plugin, which might be
	// different from the package name.  The version of the plugin, which might be
	// different from the version of the package.
	PluginVersion string

	// A map containing overrides for module names to package names.
	ModuleToPackage map[string]string

	// An indicator for whether the package contains enums.
	ContainsEnums bool

	// A map allowing you to map the name of a provider to the name of the module encapsulating the provider.
	ProviderNameToModuleName map[string]string

	// Additional files to include in TypeScript compilation. These paths are added to the `files` section of the
	// generated `tsconfig.json`. A typical use case for this is compiling hand-authored unit test files that check
	// the generated code.
	ExtraTypeScriptFiles []string

	// Determines whether to make single-return-value methods return an output object or the single value.
	LiftSingleValueMethodReturns bool

	// Respect the Pkg.Version field in the schema
	RespectSchemaVersion bool

	// Experimental flag that permits `import type *` style code to be generated to optimize startup time of
	// programs consuming the provider by minimizing the set of Node modules loaded at startup. Turning this on may
	// currently generate non-compiling code for some providers; but if the code compiles it is safe to use. Also,
	// turning this on requires TypeScript 3.8 or higher to compile the generated code.
	UseTypeOnlyReferences bool
}

type PythonInputType string

const (
	// Use the default type generation from pulumi/pulumi.
	PythonInputTypeDefault = ""
	// Generate args classes only.
	PythonInputTypeClasses = "classes"
	// Generate TypedDicts side-by-side with args classes.
	PythonInputTypeClassesAndDicts = "classes-and-dicts"
)

// Python contains optional overlay information for Python code-generation.
type Python struct {
	Requires      map[string]string // Pip install_requires information.
	Overlay       *Overlay          // optional overlay information for augmented code-generation.
	UsesIOClasses bool              // Deprecated: No longer required, all providers use IO classes.
	PackageName   string            // Name of the Python package to generate

	// PythonRequires determines the Python versions that the generated provider supports
	PythonRequires string

	// Optional overrides for Pulumi module names
	//
	//    { "flowcontrol.apiserver.k8s.io/v1alpha1": "flowcontrol/v1alpha1" }
	//
	ModuleNameOverrides map[string]string

	// Determines whether to make single-return-value methods return an output object or the single value.
	LiftSingleValueMethodReturns bool

	// Respect the Pkg.Version field for emitted code.
	RespectSchemaVersion bool

	// If enabled, a pyproject.toml file will be generated.
	PyProject struct {
		Enabled bool
	}

	// Specifies what types are used for inputs.
	InputTypes PythonInputType
}

// Golang contains optional overlay information for Golang code-generation.
type Golang struct {
	GenerateResourceContainerTypes bool     // Generate container types for resources e.g. arrays, maps, pointers etc.
	ImportBasePath                 string   // Base import path for package.
	Overlay                        *Overlay // optional overlay information for augmented code-generation.

	// Module path for go.mod
	//
	//   go get github.com/pulumi/pulumi-aws-native/sdk/go/aws@v0.16.0
	//          ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ module path
	//                                                  ~~~~~~ package path - can be any number of path parts
	//                                                         ~~~~~~~ version
	ModulePath string

	// Explicit package name, which may be different to the import path.
	RootPackageName string

	// Map from module -> package name
	//
	//    { "flowcontrol.apiserver.k8s.io/v1alpha1": "flowcontrol/v1alpha1" }
	//
	ModuleToPackage map[string]string

	// Map from package name -> package alias
	//
	//    { "github.com/pulumi/pulumi-kubernetes/sdk/go/kubernetes/flowcontrol/v1alpha1": "flowcontrolv1alpha1" }
	//
	PackageImportAliases map[string]string

	// The version of the Pulumi SDK used with this provider, e.g. 3.
	// Used to generate doc links for pulumi builtin types. If omitted, the latest SDK version is used.
	PulumiSDKVersion int

	// Feature flag to disable generating `$fnOutput` invoke
	// function versions to save space.
	DisableFunctionOutputVersions bool

	// Determines whether to make single-return-value methods return an output struct or the value.
	LiftSingleValueMethodReturns bool

	// Feature flag to disable generating input type registration. This is a
	// space saving measure.
	DisableInputTypeRegistrations bool

	// Feature flag to disable generating Pulumi object default functions. This is a
	// space saving measure.
	DisableObjectDefaults bool

	// GenerateExtraInputTypes determines whether or not the code generator generates input (and output) types for
	// all plain types, instead of for only types that are used as input/output types.
	GenerateExtraInputTypes bool

	// omitExtraInputTypes determines whether the code generator generates input (and output) types
	// for all plain types, instead of for only types that are used as input/output types.
	OmitExtraInputTypes bool

	// Respect the Pkg.Version field for emitted code.
	RespectSchemaVersion bool

	// InternalDependencies are blank imports that are emitted in the SDK so that `go mod tidy` does not remove the
	// associated module dependencies from the SDK's go.mod.
	InternalDependencies []string

	// Specifies how to handle generating a variant of the SDK that uses generics.
	// Allowed values are the following:
	// - "none" (default): do not generate a generics variant of the SDK
	// - "side-by-side": generate a side-by-side generics variant of the SDK under the x subdirectory
	// - "only-generics": generate a generics variant of the SDK only
	Generics string
}

// CSharp contains optional overlay information for C# code-generation.
type CSharp struct {
	PackageReferences map[string]string // NuGet package reference information.
	Overlay           *Overlay          // optional overlay information for augmented code-generation.
	Namespaces        map[string]string // Known .NET namespaces with proper capitalization.
	RootNamespace     string            // The root namespace if setting to something other than Pulumi in the package name

	Compatibility          string
	DictionaryConstructors bool
	ProjectReferences      []string

	// Determines whether to make single-return-value methods return an output object or the single value.
	LiftSingleValueMethodReturns bool

	// Allow the Pkg.Version field to filter down to emitted code.
	RespectSchemaVersion bool
}

// Java contains optional overlay information for Java code-generation.
type Java struct {
	BasePackage string   // the Base package for the Java SDK
	Overlay     *Overlay // optional overlay information for augmented code-generation.
	// If set to "gradle" enables a generation of a basic set of
	// Gradle build files.
	BuildFiles string

	// If non-empty and BuildFiles="gradle", enables the use of a
	// given version of io.github.gradle-nexus.publish-plugin in
	// the generated Gradle build files.
	GradleNexusPublishPluginVersion string

	Packages     map[string]string `json:"packages,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	GradleTest   string            `json:"gradleTest"`
}

// PreConfigureCallback is a function to invoke prior to calling the TF provider Configure
type PreConfigureCallback func(vars resource.PropertyMap, config shim.ResourceConfig) error

// PreConfigureCallbackWithLogger is a function to invoke prior to calling the T
type PreConfigureCallbackWithLogger func(
	ctx context.Context,
	host *provider.HostClient, vars resource.PropertyMap,
	config shim.ResourceConfig,
) error

// The types below are marshallable versions of the schema descriptions associated with a provider. These are used when
// marshalling a provider info as JSON; Note that these types only represent a subset of the information associated
// with a ProviderInfo; thus, a ProviderInfo cannot be round-tripped through JSON.

// MarshallableSchemaShim is the JSON-marshallable form of a Terraform schema.
type MarshallableSchemaShim struct {
	Type               shim.ValueType        `json:"type"`
	Optional           bool                  `json:"optional,omitempty"`
	Required           bool                  `json:"required,omitempty"`
	Computed           bool                  `json:"computed,omitempty"`
	ForceNew           bool                  `json:"forceNew,omitempty"`
	Elem               *MarshallableElemShim `json:"element,omitempty"`
	MaxItems           int                   `json:"maxItems,omitempty"`
	MinItems           int                   `json:"minItems,omitempty"`
	DeprecationMessage string                `json:"deprecated,omitempty"`
}

// MarshalSchemaShim converts a Terraform schema into a MarshallableSchema.
func MarshalSchemaShim(s shim.Schema) *MarshallableSchemaShim {
	return &MarshallableSchemaShim{
		Type:               s.Type(),
		Optional:           s.Optional(),
		Required:           s.Required(),
		Computed:           s.Computed(),
		ForceNew:           s.ForceNew(),
		Elem:               MarshalElemShim(s.Elem()),
		MaxItems:           s.MaxItems(),
		MinItems:           s.MinItems(),
		DeprecationMessage: s.Deprecated(),
	}
}

// Unmarshal creates a mostly-initialized Terraform schema from the given MarshallableSchemaShim.
func (m *MarshallableSchemaShim) Unmarshal() shim.Schema {
	return (&schema.Schema{
		Type:       m.Type,
		Optional:   m.Optional,
		Required:   m.Required,
		Computed:   m.Computed,
		ForceNew:   m.ForceNew,
		Elem:       m.Elem.Unmarshal(),
		MaxItems:   m.MaxItems,
		MinItems:   m.MinItems,
		Deprecated: m.DeprecationMessage,
	}).Shim()
}

// MarshallableResourceShim is the JSON-marshallable form of a Terraform resource schema.
type MarshallableResourceShim map[string]*MarshallableSchemaShim

// MarshalResourceShim converts a Terraform resource schema into a MarshallableResourceShim.
func MarshalResourceShim(r shim.Resource) MarshallableResourceShim {
	m := make(MarshallableResourceShim)
	if r.Schema() == nil {
		return m
	}
	r.Schema().Range(func(k string, v shim.Schema) bool {
		m[k] = MarshalSchemaShim(v)
		return true
	})
	return m
}

// Unmarshal creates a mostly-initialized Terraform resource schema from the given MarshallableResourceShim.
func (m MarshallableResourceShim) Unmarshal() shim.Resource {
	s := schema.SchemaMap{}
	for k, v := range m {
		s[k] = v.Unmarshal()
	}
	return (&schema.Resource{Schema: s}).Shim()
}

// MarshallableElemShim is the JSON-marshallable form of a Terraform schema's element field.
type MarshallableElemShim struct {
	Schema   *MarshallableSchemaShim  `json:"schema,omitempty"`
	Resource MarshallableResourceShim `json:"resource,omitempty"`
}

// MarshalElemShim converts a Terraform schema's element field into a MarshallableElemShim.
func MarshalElemShim(e interface{}) *MarshallableElemShim {
	switch v := e.(type) {
	case shim.Schema:
		return &MarshallableElemShim{Schema: MarshalSchemaShim(v)}
	case shim.Resource:
		return &MarshallableElemShim{Resource: MarshalResourceShim(v)}
	default:
		contract.Assertf(e == nil, "unexpected schema element of type %T", e)
		return nil
	}
}

// Unmarshal creates a Terraform schema element from a MarshallableElemShim.
func (m *MarshallableElemShim) Unmarshal() interface{} {
	switch {
	case m == nil:
		return nil
	case m.Schema != nil:
		return m.Schema.Unmarshal()
	default:
		// m.Resource might be nil in which case it was empty when marshalled. But Unmarshal can be called on
		// nil and returns something sensible.
		return m.Resource.Unmarshal()
	}
}

// MarshallableProviderShim is the JSON-marshallable form of a Terraform provider schema.
type MarshallableProviderShim struct {
	Schema      map[string]*MarshallableSchemaShim  `json:"schema,omitempty"`
	Resources   map[string]MarshallableResourceShim `json:"resources,omitempty"`
	DataSources map[string]MarshallableResourceShim `json:"dataSources,omitempty"`
}

// MarshalProviderShim converts a Terraform provider schema into a MarshallableProviderShim.
func MarshalProviderShim(p shim.Provider) *MarshallableProviderShim {
	if p == nil {
		return nil
	}

	config := make(map[string]*MarshallableSchemaShim)
	p.Schema().Range(func(k string, v shim.Schema) bool {
		config[k] = MarshalSchemaShim(v)
		return true
	})
	resources := make(map[string]MarshallableResourceShim)
	p.ResourcesMap().Range(func(k string, v shim.Resource) bool {
		resources[k] = MarshalResourceShim(v)
		return true
	})
	dataSources := make(map[string]MarshallableResourceShim)
	p.DataSourcesMap().Range(func(k string, v shim.Resource) bool {
		dataSources[k] = MarshalResourceShim(v)
		return true
	})
	return &MarshallableProviderShim{
		Schema:      config,
		Resources:   resources,
		DataSources: dataSources,
	}
}

// Unmarshal creates a mostly-initialized Terraform provider schema from a MarshallableProvider
func (m *MarshallableProviderShim) Unmarshal() shim.Provider {
	if m == nil {
		return nil
	}

	config := schema.SchemaMap{}
	for k, v := range m.Schema {
		config[k] = v.Unmarshal()
	}
	resources := schema.ResourceMap{}
	for k, v := range m.Resources {
		resources[k] = v.Unmarshal()
	}
	dataSources := schema.ResourceMap{}
	for k, v := range m.DataSources {
		dataSources[k] = v.Unmarshal()
	}
	return (&schema.Provider{
		Schema:         config,
		ResourcesMap:   resources,
		DataSourcesMap: dataSources,
	}).Shim()
}

// MarshallableSchema is the JSON-marshallable form of a Pulumi SchemaInfo value.
type MarshallableSchema struct {
	Name        string                         `json:"name,omitempty"`
	CSharpName  string                         `json:"csharpName,omitempty"`
	Type        tokens.Type                    `json:"typeomitempty"`
	AltTypes    []tokens.Type                  `json:"altTypes,omitempty"`
	Elem        *MarshallableSchema            `json:"element,omitempty"`
	Fields      map[string]*MarshallableSchema `json:"fields,omitempty"`
	Asset       *AssetTranslation              `json:"asset,omitempty"`
	Default     *MarshallableDefault           `json:"default,omitempty"`
	MaxItemsOne *bool                          `json:"maxItemsOne,omitempty"`
	Deprecated  string                         `json:"deprecated,omitempty"`
	ForceNew    *bool                          `json:"forceNew,omitempty"`
	Secret      *bool                          `json:"secret,omitempty"`
}

// MarshalSchema converts a Pulumi SchemaInfo value into a MarshallableSchemaInfo value.
func MarshalSchema(s *Schema) *MarshallableSchema {
	if s == nil {
		return nil
	}

	fields := make(map[string]*MarshallableSchema)
	for k, v := range s.Fields {
		fields[k] = MarshalSchema(v)
	}
	return &MarshallableSchema{
		Name:        s.Name,
		CSharpName:  s.CSharpName,
		Type:        s.Type,
		AltTypes:    s.AltTypes,
		Elem:        MarshalSchema(s.Elem),
		Fields:      fields,
		Asset:       s.Asset,
		Default:     MarshalDefault(s.Default),
		MaxItemsOne: s.MaxItemsOne,
		Deprecated:  s.DeprecationMessage,
		ForceNew:    s.ForceNew,
		Secret:      s.Secret,
	}
}

// Unmarshal creates a mostly-=initialized Pulumi SchemaInfo value from the given MarshallableSchemaInfo.
func (m *MarshallableSchema) Unmarshal() *Schema {
	if m == nil {
		return nil
	}

	fields := make(map[string]*Schema)
	for k, v := range m.Fields {
		fields[k] = v.Unmarshal()
	}
	return &Schema{
		Name:               m.Name,
		CSharpName:         m.CSharpName,
		Type:               m.Type,
		AltTypes:           m.AltTypes,
		Elem:               m.Elem.Unmarshal(),
		Fields:             fields,
		Asset:              m.Asset,
		Default:            m.Default.Unmarshal(),
		MaxItemsOne:        m.MaxItemsOne,
		DeprecationMessage: m.Deprecated,
		ForceNew:           m.ForceNew,
		Secret:             m.Secret,
	}
}

// MarshallableDefault is the JSON-marshallable form of a Pulumi [Default] value.
type MarshallableDefault struct {
	AutoNamed bool        `json:"autonamed,omitempty"`
	IsFunc    bool        `json:"isFunc,omitempty"`
	Value     interface{} `json:"value,omitempty"`
	EnvVars   []string    `json:"envvars,omitempty"`
}

// MarshalDefault converts a Pulumi DefaultInfo value into a [MarshallableDefault] value.
func MarshalDefault(d *Default) *MarshallableDefault {
	if d == nil {
		return nil
	}

	return &MarshallableDefault{
		AutoNamed: d.AutoNamed,
		IsFunc:    d.From != nil || d.ComputeDefault != nil,
		Value:     d.Value,
		EnvVars:   d.EnvVars,
	}
}

// Unmarshal creates a mostly-initialized Pulumi [Default] value from the given [MarshallableDefault].
func (m *MarshallableDefault) Unmarshal() *Default {
	if m == nil {
		return nil
	}

	defInfo := &Default{
		AutoNamed: m.AutoNamed,
		Value:     m.Value,
		EnvVars:   m.EnvVars,
	}

	if m.IsFunc {
		defInfo.ComputeDefault = func(context.Context, ComputeDefaultOptions) (interface{}, error) {
			panic("transforms cannot be run on unmarshaled Default values")
		}
	}
	return defInfo
}

// MarshallableResource is the JSON-marshallable form of a Pulumi ResourceInfo value.
type MarshallableResource struct {
	Tok        tokens.Type                    `json:"tok"`
	CSharpName string                         `json:"csharpName,omitempty"`
	Fields     map[string]*MarshallableSchema `json:"fields"`

	// Deprecated: IDFields is not currently used and will be deprecated in the next major version of
	// pulumi-terraform-bridge.
	IDFields []string `json:"idFields"`
}

// MarshalResource converts a Pulumi ResourceInfo value into a MarshallableResourceInfo value.
func MarshalResource(r *Resource) *MarshallableResource {
	fields := make(map[string]*MarshallableSchema)
	for k, v := range r.Fields {
		fields[k] = MarshalSchema(v)
	}
	return &MarshallableResource{
		Tok:        r.Tok,
		CSharpName: r.CSharpName,
		Fields:     fields,
		IDFields:   r.IDFields,
	}
}

// Unmarshal creates a mostly-=initialized Pulumi ResourceInfo value from the given MarshallableResourceInfo.
func (m *MarshallableResource) Unmarshal() *Resource {
	fields := make(map[string]*Schema)
	for k, v := range m.Fields {
		fields[k] = v.Unmarshal()
	}
	return &Resource{
		Tok:        m.Tok,
		Fields:     fields,
		IDFields:   m.IDFields,
		CSharpName: m.CSharpName,
	}
}

// MarshallableDataSource is the JSON-marshallable form of a Pulumi DataSourceInfo value.
type MarshallableDataSource struct {
	Tok    tokens.ModuleMember            `json:"tok"`
	Fields map[string]*MarshallableSchema `json:"fields"`
}

// MarshalDataSource converts a Pulumi DataSourceInfo value into a MarshallableDataSourceInfo value.
func MarshalDataSource(d *DataSource) *MarshallableDataSource {
	fields := make(map[string]*MarshallableSchema)
	for k, v := range d.Fields {
		fields[k] = MarshalSchema(v)
	}
	return &MarshallableDataSource{
		Tok:    d.Tok,
		Fields: fields,
	}
}

// Unmarshal creates a mostly-=initialized Pulumi DataSourceInfo value from the given MarshallableDataSourceInfo.
func (m *MarshallableDataSource) Unmarshal() *DataSource {
	fields := make(map[string]*Schema)
	for k, v := range m.Fields {
		fields[k] = v.Unmarshal()
	}
	return &DataSource{
		Tok:    m.Tok,
		Fields: fields,
	}
}

// MarshallableProvider is the JSON-marshallable form of a Pulumi ProviderInfo value.
type MarshallableProvider struct {
	Provider          *MarshallableProviderShim          `json:"provider"`
	Name              string                             `json:"name"`
	Version           string                             `json:"version"`
	Config            map[string]*MarshallableSchema     `json:"config,omitempty"`
	Resources         map[string]*MarshallableResource   `json:"resources,omitempty"`
	DataSources       map[string]*MarshallableDataSource `json:"dataSources,omitempty"`
	TFProviderVersion string                             `json:"tfProviderVersion,omitempty"`
}

// MarshalProvider converts a Pulumi ProviderInfo value into a MarshallableProviderInfo value.
func MarshalProvider(p *Provider) *MarshallableProvider {
	config := make(map[string]*MarshallableSchema)
	for k, v := range p.Config {
		config[k] = MarshalSchema(v)
	}
	resources := make(map[string]*MarshallableResource)
	for k, v := range p.Resources {
		resources[k] = MarshalResource(v)
	}
	dataSources := make(map[string]*MarshallableDataSource)
	for k, v := range p.DataSources {
		dataSources[k] = MarshalDataSource(v)
	}

	info := MarshallableProvider{
		Provider:          MarshalProviderShim(p.P),
		Name:              p.Name,
		Version:           p.Version,
		Config:            config,
		Resources:         resources,
		DataSources:       dataSources,
		TFProviderVersion: p.TFProviderVersion,
	}

	return &info
}

// Unmarshal creates a mostly-=initialized Pulumi ProviderInfo value from the given MarshallableProviderInfo.
func (m *MarshallableProvider) Unmarshal() *Provider {
	config := make(map[string]*Schema)
	for k, v := range m.Config {
		config[k] = v.Unmarshal()
	}
	resources := make(map[string]*Resource)
	for k, v := range m.Resources {
		resources[k] = v.Unmarshal()
	}
	dataSources := make(map[string]*DataSource)
	for k, v := range m.DataSources {
		dataSources[k] = v.Unmarshal()
	}

	info := Provider{
		P:                 m.Provider.Unmarshal(),
		Name:              m.Name,
		Version:           m.Version,
		Config:            config,
		Resources:         resources,
		DataSources:       dataSources,
		TFProviderVersion: m.TFProviderVersion,
	}

	return &info
}

// If specified, the hook will run just prior to executing Terraform state upgrades to transform the resource state as
// stored in Pulumi. It can be used to perform idempotent corrections on corrupt state and to compensate for
// Terraform-level state upgrade not working as expected. Returns the corrected resource state and version. To be used
// with care.
//
// See also: https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/resource/schema#Schema.Version
type PreStateUpgradeHook = func(PreStateUpgradeHookArgs) (int64, resource.PropertyMap, error)

type PreStateUpgradeHookArgs struct {
	PriorState              resource.PropertyMap
	PriorStateSchemaVersion int64
	ResourceSchemaVersion   int64
}
