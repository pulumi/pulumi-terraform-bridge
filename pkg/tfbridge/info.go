// Copyright 2016-2018, Pulumi Corporation.
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

package tfbridge

import (
	"fmt"
	"unicode"

	"github.com/blang/semver"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

const (
	MPL20LicenseType          TFProviderLicense = "MPL 2.0"
	MITLicenseType            TFProviderLicense = "MIT"
	Apache20LicenseType       TFProviderLicense = "Apache 2.0"
	UnlicensedLicenseType     TFProviderLicense = "UNLICENSED"
	DataSourceNoImportDefault string            = ""
)

// ProviderInfo contains information about a Terraform provider plugin that we will use to generate the Pulumi
// metadata.  It primarily contains a pointer to the Terraform schema, but can also contain specific name translations.
//
//nolint: lll
type ProviderInfo struct {
	P              shim.Provider                      // the TF provider/schema.
	Name           string                             // the TF provider name (e.g. terraform-provider-XXXX).
	ResourcePrefix string                             // the prefix on resources the provider exposes, if different to `Name`.
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
	Config         map[string]*SchemaInfo             // a map of TF name to config schema overrides.
	ExtraConfig    map[string]*ConfigInfo             // a list of Pulumi-only configuration variables.
	Resources      map[string]*ResourceInfo           // a map of TF name to Pulumi name; standard mangling occurs if no entry.
	DataSources    map[string]*DataSourceInfo         // a map of TF name to Pulumi resource info.
	ExtraTypes     map[string]pschema.ComplexTypeSpec // a map of Pulumi token to schema type for overlaid types.
	// ExtraResourceHclExamples is a slice of additional HCL examples attached to resources which are converted to the
	// relevant target language(s)
	ExtraResourceHclExamples []HclExampler
	// ExtraFunctionHclExamples is a slice of additional HCL examples attached to functions which are converted to the
	// relevant target language(s)
	ExtraFunctionHclExamples []HclExampler
	IgnoreMappings           []string           // a list of TF resources and data sources to ignore in mappings errors
	PluginDownloadURL        string             // an optional URL to download the provider binary from.
	JavaScript               *JavaScriptInfo    // optional overlay information for augmented JavaScript code-generation.
	Python                   *PythonInfo        // optional overlay information for augmented Python code-generation.
	Golang                   *GolangInfo        // optional overlay information for augmented Golang code-generation.
	CSharp                   *CSharpInfo        // optional overlay information for augmented C# code-generation.
	TFProviderVersion        string             // the version of the TF provider on which this was based
	TFProviderLicense        *TFProviderLicense // license that the TF provider is distributed under. Default `MPL 2.0`.
	TFProviderModuleVersion  string             // the Go module version of the provider. Default is unversioned e.g. v1

	PreConfigureCallback PreConfigureCallback // a provider-specific callback to invoke prior to TF Configure
}

// TFProviderLicense is a way to be able to pass a license type for the upstream Terraform provider.
type TFProviderLicense string

// GetResourcePrefix returns the resource prefix for the provider: info.ResourcePrefix
// if that is set, or info.Name if not. This is to avoid unexpected behaviour with providers
// which have no need to set ResourcePrefix following its introduction.
func (info ProviderInfo) GetResourcePrefix() string {
	if info.ResourcePrefix == "" {
		return info.Name
	}

	return info.ResourcePrefix
}

func (info ProviderInfo) GetGitHubOrg() string {
	if info.GitHubOrg == "" {
		return "terraform-providers"
	}

	return info.GitHubOrg
}

func (info ProviderInfo) GetGitHubHost() string {
	if info.GitHubHost == "" {
		return "github.com"
	}

	return info.GitHubHost
}

func (info ProviderInfo) GetTFProviderLicense() TFProviderLicense {
	if info.TFProviderLicense == nil {
		return MPL20LicenseType
	}

	return *info.TFProviderLicense
}

func (info ProviderInfo) GetProviderModuleVersion() string {
	if info.TFProviderModuleVersion == "" {
		return "" // there is no such thing as a v1 module - there is just a missing version declaration
	}

	return info.TFProviderModuleVersion
}

// AliasInfo is a partial description of prior named used for a resource. It can be processed in the
// context of a resource creation to determine what the full aliased URN would be.
//
// It can be used by Pulumi resource providers to change the aspects of it (i.e. what module it is
// contained in), without causing resources to be recreated for customers who migrate from the
// original resource to the current resource.
type AliasInfo struct {
	Name    *string
	Type    *string
	Project *string
}

// ResourceOrDataSourceInfo is a shared interface to ResourceInfo and DataSourceInfo mappings
type ResourceOrDataSourceInfo interface {
	GetTok() tokens.Token              // a type token to override the default; "" uses the default.
	GetFields() map[string]*SchemaInfo // a map of custom field names; if a type is missing, uses the default.
	GetDocs() *DocInfo                 // overrides for finding and mapping TF docs.
	GetImportDetails() string          // Get overwritten instruction for importing a resource
}

// ResourceInfo is a top-level type exported by a provider.  This structure can override the type to generate.  It can
// also give custom metadata for fields, using the SchemaInfo structure below.  Finally, a set of composite keys can be
// given; this is used when Terraform needs more than just the ID to uniquely identify and query for a resource.
type ResourceInfo struct {
	Tok                 tokens.Type            // a type token to override the default; "" uses the default.
	Fields              map[string]*SchemaInfo // a map of custom field names; if a type is missing, uses the default.
	IDFields            []string               // an optional list of ID alias fields.
	Docs                *DocInfo               // overrides for finding and mapping TF docs.
	DeleteBeforeReplace bool                   // if true, Pulumi will delete before creating new replacement resources.
	Aliases             []AliasInfo            // aliases for this resources, if any.
	DeprecationMessage  string                 // message to use in deprecation warning
	CSharpName          string                 // .NET-specific name
	ImportDetails       string                 // Docs overwrite for import instructions
}

// GetTok returns a resource type token
func (info *ResourceInfo) GetTok() tokens.Token { return tokens.Token(info.Tok) }

// GetFields returns information about the resource's custom fields
func (info *ResourceInfo) GetFields() map[string]*SchemaInfo { return info.Fields }

// GetDocs returns a resource docs override from the Pulumi provider
func (info *ResourceInfo) GetDocs() *DocInfo { return info.Docs }

// GetImportDetails returns a string of import instructions defined in the Pulumi provider. Defaults to empty.
func (info *ResourceInfo) GetImportDetails() string { return info.ImportDetails }

// DataSourceInfo can be used to override a data source's standard name mangling and argument/return information.
type DataSourceInfo struct {
	Tok                tokens.ModuleMember
	Fields             map[string]*SchemaInfo
	Docs               *DocInfo // overrides for finding and mapping TF docs.
	DeprecationMessage string   // message to use in deprecation warning
}

// GetTok returns a datasource type token
func (info *DataSourceInfo) GetTok() tokens.Token { return tokens.Token(info.Tok) }

// GetFields returns information about the datasource's custom fields
func (info *DataSourceInfo) GetFields() map[string]*SchemaInfo { return info.Fields }

// GetDocs returns a datasource docs override from the Pulumi provider
func (info *DataSourceInfo) GetDocs() *DocInfo { return info.Docs }

// GetImportDetails always returns empty, as datasources never get imported. Satisfies the ResourceOrDataSourceInfo interface.
func (info *DataSourceInfo) GetImportDetails() string { return DataSourceNoImportDefault } // DataSources cannot be imported so this field defaults to empty

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
	Transform Transformer

	// a schema override for elements for arrays, maps, and sets.
	Elem *SchemaInfo

	// a map of custom field names; if a type is missing, the default is used.
	Fields map[string]*SchemaInfo

	// a map of asset translation information, if this is an asset.
	Asset *AssetTranslation

	// an optional default directive to be applied if a value is missing.
	Default *DefaultInfo

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

// ConfigInfo represents a synthetic configuration variable that is Pulumi-only, and not passed to Terraform.
type ConfigInfo struct {
	// Info is the Pulumi schema for this variable.
	Info *SchemaInfo
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

// DocInfo contains optional overrides for finding and mapping TF docs.
type DocInfo struct {
	Source                         string // an optional override to locate TF docs; "" uses the default.
	Markdown                       []byte // an optional override for the source markdown.
	IncludeAttributesFrom          string // optionally include attributes from another raw resource for docs.
	IncludeArgumentsFrom           string // optionally include arguments from another raw resource for docs.
	IncludeAttributesFromArguments string // optionally include attributes from another raw resource's arguments.
}

// HasDefault returns true if there is a default value for this property.
func (info SchemaInfo) HasDefault() bool {
	return info.Default != nil
}

// DefaultInfo lets fields get default values at runtime, before they are even passed to Terraform.
type DefaultInfo struct {
	// AutoNamed is true if this default represents an autogenerated name.
	AutoNamed bool
	// Config uses a configuration variable from this package as the default value.
	Config string
	// From applies a transformation from other resource properties.
	From func(res *PulumiResource) (interface{}, error)
	// Value injects a raw literal value as the default.
	Value interface{}
	// EnvVars to use for defaults. If none of these variables have values at runtime, the value of `Value` (if any)
	// will be used as the default.
	EnvVars []string
}

// PulumiResource is just a little bundle that carries URN and properties around.
type PulumiResource struct {
	URN        resource.URN
	Properties resource.PropertyMap
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
}

// PreConfigureCallback is a function to invoke prior to calling the TF provider Configure
type PreConfigureCallback func(vars resource.PropertyMap, config shim.ResourceConfig) error

// The types below are marshallable versions of the schema descriptions associated with a provider. These are used when
// marshalling a provider info as JSON; Note that these types only represent a subset of the informatino associated
// with a ProviderInfo; thus, a ProviderInfo cannot be round-tripped through JSON.

// MarshallableSchema is the JSON-marshallable form of a Terraform schema.
type MarshallableSchema struct {
	Type               shim.ValueType    `json:"type"`
	Optional           bool              `json:"optional,omitempty"`
	Required           bool              `json:"required,omitempty"`
	Computed           bool              `json:"computed,omitempty"`
	ForceNew           bool              `json:"forceNew,omitempty"`
	Elem               *MarshallableElem `json:"element,omitempty"`
	MaxItems           int               `json:"maxItems,omitempty"`
	MinItems           int               `json:"minItems,omitempty"`
	DeprecationMessage string            `json:"deprecated,omitempty"`
}

// MarshalSchema converts a Terraform schema into a MarshallableSchema.
func MarshalSchema(s shim.Schema) *MarshallableSchema {
	return &MarshallableSchema{
		Type:               s.Type(),
		Optional:           s.Optional(),
		Required:           s.Required(),
		Computed:           s.Computed(),
		ForceNew:           s.ForceNew(),
		Elem:               MarshalElem(s.Elem()),
		MaxItems:           s.MaxItems(),
		MinItems:           s.MinItems(),
		DeprecationMessage: s.Deprecated(),
	}
}

// Unmarshal creates a mostly-initialized Terraform schema from the given MarshallableSchema.
func (m *MarshallableSchema) Unmarshal() shim.Schema {
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

// MarshallableResource is the JSON-marshallable form of a Terraform resource schema.
type MarshallableResource map[string]*MarshallableSchema

// MarshalResource converts a Terraform resource schema into a MarshallableResource.
func MarshalResource(r shim.Resource) MarshallableResource {
	m := make(MarshallableResource)
	r.Schema().Range(func(k string, v shim.Schema) bool {
		m[k] = MarshalSchema(v)
		return true
	})
	return m
}

// Unmarshal creates a mostly-initialized Terraform resource schema from the given MarshallableResource.
func (m MarshallableResource) Unmarshal() shim.Resource {
	s := schema.SchemaMap{}
	for k, v := range m {
		s[k] = v.Unmarshal()
	}
	return (&schema.Resource{Schema: s}).Shim()
}

// MarshallableElem is the JSON-marshallable form of a Terraform schema's element field.
type MarshallableElem struct {
	Schema   *MarshallableSchema  `json:"schema,omitempty"`
	Resource MarshallableResource `json:"resource,omitempty"`
}

// MarshalElem converts a Terraform schema's element field into a MarshallableElem.
func MarshalElem(e interface{}) *MarshallableElem {
	switch v := e.(type) {
	case shim.Schema:
		return &MarshallableElem{Schema: MarshalSchema(v)}
	case shim.Resource:
		return &MarshallableElem{Resource: MarshalResource(v)}
	default:
		contract.Assertf(e == nil, "unexpected schema element of type %T", e)
		return nil
	}
}

// Unmarshal creates a Terraform schema element from a MarshallableElem.
func (m *MarshallableElem) Unmarshal() interface{} {
	switch {
	case m == nil:
		return nil
	case m.Schema != nil:
		return m.Schema.Unmarshal()
	case m.Resource != nil:
		return m.Resource.Unmarshal()
	default:
		return nil
	}
}

// MarshallableProvider is the JSON-marshallable form of a Terraform provider schema.
type MarshallableProvider struct {
	Schema      map[string]*MarshallableSchema  `json:"schema,omitempty"`
	Resources   map[string]MarshallableResource `json:"resources,omitempty"`
	DataSources map[string]MarshallableResource `json:"dataSources,omitempty"`
}

// MarshalProvider converts a Terraform provider schema into a MarshallableProvider.
func MarshalProvider(p shim.Provider) *MarshallableProvider {
	if p == nil {
		return nil
	}

	config := make(map[string]*MarshallableSchema)
	p.Schema().Range(func(k string, v shim.Schema) bool {
		config[k] = MarshalSchema(v)
		return true
	})
	resources := make(map[string]MarshallableResource)
	p.ResourcesMap().Range(func(k string, v shim.Resource) bool {
		resources[k] = MarshalResource(v)
		return true
	})
	dataSources := make(map[string]MarshallableResource)
	p.DataSourcesMap().Range(func(k string, v shim.Resource) bool {
		dataSources[k] = MarshalResource(v)
		return true
	})
	return &MarshallableProvider{
		Schema:      config,
		Resources:   resources,
		DataSources: dataSources,
	}
}

// Unmarshal creates a mostly-initialized Terraform provider schema from a MarshallableProvider
func (m *MarshallableProvider) Unmarshal() shim.Provider {
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

// MarshallableSchemaInfo is the JSON-marshallable form of a Pulumi SchemaInfo value.
type MarshallableSchemaInfo struct {
	Name        string                             `json:"name,omitempty"`
	CSharpName  string                             `json:"csharpName,omitempty"`
	Type        tokens.Type                        `json:"typeomitempty"`
	AltTypes    []tokens.Type                      `json:"altTypes,omitempty"`
	Elem        *MarshallableSchemaInfo            `json:"element,omitempty"`
	Fields      map[string]*MarshallableSchemaInfo `json:"fields,omitempty"`
	Asset       *AssetTranslation                  `json:"asset,omitempty"`
	Default     *MarshallableDefaultInfo           `json:"default,omitempty"`
	MaxItemsOne *bool                              `json:"maxItemsOne,omitempty"`
	Deprecated  string                             `json:"deprecated,omitempty"`
	ForceNew    *bool                              `json:"forceNew,omitempty"`
	Secret      *bool                              `json:"secret,omitempty"`
}

// MarshalSchemaInfo converts a Pulumi SchemaInfo value into a MarshallableSchemaInfo value.
func MarshalSchemaInfo(s *SchemaInfo) *MarshallableSchemaInfo {
	if s == nil {
		return nil
	}

	fields := make(map[string]*MarshallableSchemaInfo)
	for k, v := range s.Fields {
		fields[k] = MarshalSchemaInfo(v)
	}
	return &MarshallableSchemaInfo{
		Name:        s.Name,
		CSharpName:  s.CSharpName,
		Type:        s.Type,
		AltTypes:    s.AltTypes,
		Elem:        MarshalSchemaInfo(s.Elem),
		Fields:      fields,
		Asset:       s.Asset,
		Default:     MarshalDefaultInfo(s.Default),
		MaxItemsOne: s.MaxItemsOne,
		Deprecated:  s.DeprecationMessage,
		ForceNew:    s.ForceNew,
		Secret:      s.Secret,
	}
}

// Unmarshal creates a mostly-=initialized Pulumi SchemaInfo value from the given MarshallableSchemaInfo.
func (m *MarshallableSchemaInfo) Unmarshal() *SchemaInfo {
	if m == nil {
		return nil
	}

	fields := make(map[string]*SchemaInfo)
	for k, v := range m.Fields {
		fields[k] = v.Unmarshal()
	}
	return &SchemaInfo{
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

// MarshallableDefaultInfo is the JSON-marshallable form of a Pulumi DefaultInfo value.
type MarshallableDefaultInfo struct {
	AutoNamed bool        `json:"autonamed,omitempty"`
	IsFunc    bool        `json:"isFunc,omitempty"`
	Value     interface{} `json:"value,omitempty"`
	EnvVars   []string    `json:"envvars,omitempty"`
}

// MarshalDefaultInfo converts a Pulumi DefaultInfo value into a MarshallableDefaultInfo value.
func MarshalDefaultInfo(d *DefaultInfo) *MarshallableDefaultInfo {
	if d == nil {
		return nil
	}

	return &MarshallableDefaultInfo{
		AutoNamed: d.AutoNamed,
		IsFunc:    d.From != nil,
		Value:     d.Value,
		EnvVars:   d.EnvVars,
	}
}

// Unmarshal creates a mostly-initialized Pulumi DefaultInfo value from the given MarshallableDefaultInfo.
func (m *MarshallableDefaultInfo) Unmarshal() *DefaultInfo {
	if m == nil {
		return nil
	}

	var f func(*PulumiResource) (interface{}, error)
	if m.IsFunc {
		f = func(*PulumiResource) (interface{}, error) {
			panic("transforms cannot be run on unmarshaled DefaultInfo values")
		}
	}

	return &DefaultInfo{
		AutoNamed: m.AutoNamed,
		From:      f,
		Value:     m.Value,
		EnvVars:   m.EnvVars,
	}
}

// MarshallableResourceInfo is the JSON-marshallable form of a Pulumi ResourceInfo value.
type MarshallableResourceInfo struct {
	Tok      tokens.Type                        `json:"tok"`
	Fields   map[string]*MarshallableSchemaInfo `json:"fields"`
	IDFields []string                           `json:"idFields"`
}

// MarshalResourceInfo converts a Pulumi ResourceInfo value into a MarshallableResourceInfo value.
func MarshalResourceInfo(r *ResourceInfo) *MarshallableResourceInfo {
	fields := make(map[string]*MarshallableSchemaInfo)
	for k, v := range r.Fields {
		fields[k] = MarshalSchemaInfo(v)
	}
	return &MarshallableResourceInfo{
		Tok:      r.Tok,
		Fields:   fields,
		IDFields: r.IDFields,
	}
}

// Unmarshal creates a mostly-=initialized Pulumi ResourceInfo value from the given MarshallableResourceInfo.
func (m *MarshallableResourceInfo) Unmarshal() *ResourceInfo {
	fields := make(map[string]*SchemaInfo)
	for k, v := range m.Fields {
		fields[k] = v.Unmarshal()
	}
	return &ResourceInfo{
		Tok:      m.Tok,
		Fields:   fields,
		IDFields: m.IDFields,
	}
}

// MarshallableDataSourceInfo is the JSON-marshallable form of a Pulumi DataSourceInfo value.
type MarshallableDataSourceInfo struct {
	Tok    tokens.ModuleMember                `json:"tok"`
	Fields map[string]*MarshallableSchemaInfo `json:"fields"`
}

// MarshalDataSourceInfo converts a Pulumi DataSourceInfo value into a MarshallableDataSourceInfo value.
func MarshalDataSourceInfo(d *DataSourceInfo) *MarshallableDataSourceInfo {
	fields := make(map[string]*MarshallableSchemaInfo)
	for k, v := range d.Fields {
		fields[k] = MarshalSchemaInfo(v)
	}
	return &MarshallableDataSourceInfo{
		Tok:    d.Tok,
		Fields: fields,
	}
}

// Unmarshal creates a mostly-=initialized Pulumi DataSourceInfo value from the given MarshallableDataSourceInfo.
func (m *MarshallableDataSourceInfo) Unmarshal() *DataSourceInfo {
	fields := make(map[string]*SchemaInfo)
	for k, v := range m.Fields {
		fields[k] = v.Unmarshal()
	}
	return &DataSourceInfo{
		Tok:    m.Tok,
		Fields: fields,
	}
}

// MarshallableProviderInfo is the JSON-marshallable form of a Pulumi ProviderInfo value.
type MarshallableProviderInfo struct {
	Provider          *MarshallableProvider                  `json:"provider"`
	Name              string                                 `json:"name"`
	Version           string                                 `json:"version"`
	Config            map[string]*MarshallableSchemaInfo     `json:"config,omitempty"`
	Resources         map[string]*MarshallableResourceInfo   `json:"resources,omitempty"`
	DataSources       map[string]*MarshallableDataSourceInfo `json:"dataSources,omitempty"`
	TFProviderVersion string                                 `json:"tfProviderVersion,omitempty"`
}

// MarshalProviderInfo converts a Pulumi ProviderInfo value into a MarshallableProviderInfo value.
func MarshalProviderInfo(p *ProviderInfo) *MarshallableProviderInfo {
	config := make(map[string]*MarshallableSchemaInfo)
	for k, v := range p.Config {
		config[k] = MarshalSchemaInfo(v)
	}
	resources := make(map[string]*MarshallableResourceInfo)
	for k, v := range p.Resources {
		resources[k] = MarshalResourceInfo(v)
	}
	dataSources := make(map[string]*MarshallableDataSourceInfo)
	for k, v := range p.DataSources {
		dataSources[k] = MarshalDataSourceInfo(v)
	}

	info := MarshallableProviderInfo{
		Provider:          MarshalProvider(p.P),
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
func (m *MarshallableProviderInfo) Unmarshal() *ProviderInfo {
	config := make(map[string]*SchemaInfo)
	for k, v := range m.Config {
		config[k] = v.Unmarshal()
	}
	resources := make(map[string]*ResourceInfo)
	for k, v := range m.Resources {
		resources[k] = v.Unmarshal()
	}
	dataSources := make(map[string]*DataSourceInfo)
	for k, v := range m.DataSources {
		dataSources[k] = v.Unmarshal()
	}

	info := ProviderInfo{
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

// MakeMember manufactures a type token for the package and the given module and type.
func MakeMember(pkg string, mod string, mem string) tokens.ModuleMember {
	return tokens.ModuleMember(pkg + ":" + mod + ":" + mem)
}

// MakeType manufactures a type token for the package and the given module and type.
func MakeType(pkg string, mod string, typ string) tokens.Type {
	return tokens.Type(MakeMember(pkg, mod, typ))
}

// MakeDataSource manufactures a standard Pulumi function token given a package, module, and data source name.  It
// automatically uses the main package and names the file by simply lower casing the data source's
// first character.
func MakeDataSource(pkg string, mod string, res string) tokens.ModuleMember {
	fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
	return MakeMember(pkg, mod+"/"+fn, res)
}

// MakeResource manufactures a standard resource token given a module and resource name.  It
// automatically uses the main package and names the file by simply lower casing the resource's
// first character.
func MakeResource(pkg string, mod string, res string) tokens.Type {
	fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
	return MakeType(pkg, mod+"/"+fn, res)
}

// BoolRef returns a reference to the bool argument.
func BoolRef(b bool) *bool {
	return &b
}

// StringValue gets a string value from a property map if present, else ""
func StringValue(vars resource.PropertyMap, prop resource.PropertyKey) string {
	val, ok := vars[prop]
	if ok && val.IsString() {
		return val.StringValue()
	}
	return ""
}

// ManagedByPulumi is a default used for some managed resources, in the absence of something more meaningful.
var ManagedByPulumi = &DefaultInfo{Value: "Managed by Pulumi"}
