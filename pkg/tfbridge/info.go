// Copyright 2016-2023, Pulumi Corporation.
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
	"os"
	"strings"
	"unicode"

	"github.com/blang/semver"
	"golang.org/x/net/context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

type urnCtxKeyType struct{}

var urnCtxKey = urnCtxKeyType{}

// XWithUrn returns a copy of ctx with the resource URN value.
//
// XWithUrn is unstable and may be changed or removed in any release.
func XWithUrn(ctx context.Context, urn resource.URN) context.Context {
	return context.WithValue(ctx, urnCtxKey, urn)
}

// GetUrn gets a resource URN from context.
//
//	urn := GetUrn(ctx)
//
// GetUrn is available on any context associated with a resource.
// Calling GetUrn on a context associated with an Invoke will panic.
func GetUrn(ctx context.Context) resource.URN {
	urn := ctx.Value(urnCtxKey).(resource.URN)
	return urn
}

const (
	MPL20LicenseType      = info.MPL20LicenseType
	MITLicenseType        = info.MITLicenseType
	Apache20LicenseType   = info.Apache20LicenseType
	UnlicensedLicenseType = info.UnlicensedLicenseType
)

// ProviderInfo contains information about a Terraform provider plugin that we will use to
// generate the Pulumi metadata.  It primarily contains a pointer to the Terraform schema,
// but can also contain specific name translations.
type ProviderInfo = info.Provider

// Send logs or status logs to the user.
//
// Logged messages are pre-associated with the resource they are called from.
type Logger interface {
	Log

	// Convert to sending ephemeral status logs to the user.
	Status() Log
}

// The set of logs available to show to the user
type Log interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// Get access to the [Logger] associated with this context.
func GetLogger(ctx context.Context) Logger {
	logger := ctx.Value(logging.CtxKey)
	contract.Assertf(logger != nil, "Cannot call GetLogger on a context that is not equipped with a Logger")
	return newLoggerAdapter(logger)
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
type MakeEditRules = info.MakeEditRules

// DocRuleInfo controls file discovery and edits for any subset of docs in a provider.
type DocRuleInfo = info.DocRule

// Information for file lookup.
type DocsPathInfo = info.DocsPath

type DocsEdit = info.DocsEdit

// TFProviderLicense is a way to be able to pass a license type for the upstream Terraform provider.
type TFProviderLicense = info.TFProviderLicense

// AliasInfo is a partial description of prior named used for a resource. It can be processed in the
// context of a resource creation to determine what the full aliased URN would be.
//
// It can be used by Pulumi resource providers to change the aspects of it (i.e. what module it is
// contained in), without causing resources to be recreated for customers who migrate from the
// original resource to the current resource.
type AliasInfo = info.Alias

// ResourceOrDataSourceInfo is a shared interface to ResourceInfo and DataSourceInfo mappings
type ResourceOrDataSourceInfo = info.ResourceOrDataSource

// ResourceInfo is a top-level type exported by a provider.  This structure can override the type to generate.  It can
// also give custom metadata for fields, using the SchemaInfo structure below.  Finally, a set of composite keys can be
// given; this is used when Terraform needs more than just the ID to uniquely identify and query for a resource.
type ResourceInfo = info.Resource

type ComputeID = info.ComputeID

type PropertyTransform = info.PropertyTransform

type PreCheckCallback = info.PreCheckCallback

// DataSourceInfo can be used to override a data source's standard name mangling and argument/return information.
type DataSourceInfo = info.DataSource

// SchemaInfo contains optional name transformations to apply.
type SchemaInfo = info.Schema

// ConfigInfo represents a synthetic configuration variable that is Pulumi-only, and not passed to Terraform.
type ConfigInfo = info.Config

// Transformer is given the option to transform a value in situ before it is processed by the bridge. This
// transformation must be deterministic and idempotent, and any value produced by this transformation must
// be a legal alternative input value. A good example is a resource that accepts either a string or
// JSON-stringable map; a resource provider may opt to store the raw string, but let users pass in maps as
// a convenience mechanism, and have the transformer stringify them on the fly. This is safe to do because
// the raw string is still accepted as a possible input value.
type Transformer = info.Transformer

// DocInfo contains optional overrides for finding and mapping TF docs.
type DocInfo = info.Doc

// DefaultInfo lets fields get default values at runtime, before they are even passed to Terraform.
type DefaultInfo = info.Default

// Configures [DefaultInfo.ComputeDefault].
type ComputeDefaultOptions = info.ComputeDefaultOptions

// PulumiResource is just a little bundle that carries URN, seed and properties around.
type PulumiResource = info.PulumiResource

// OverlayInfo contains optional overlay information.  Each info has a 1:1 correspondence with a module and
// permits extra files to be included from the overlays/ directory when building up packs/.  This allows augmented
// code-generation for convenient things like helper functions, modules, and gradual typing.
type OverlayInfo = info.Overlay

// JavaScriptInfo contains optional overlay information for Python code-generation.
type JavaScriptInfo = info.JavaScript

// PythonInfo contains optional overlay information for Python code-generation.
type PythonInfo = info.Python

// GolangInfo contains optional overlay information for Golang code-generation.
type GolangInfo = info.Golang

// CSharpInfo contains optional overlay information for C# code-generation.
type CSharpInfo = info.CSharp

// See https://github.com/pulumi/pulumi-java/blob/main/pkg/codegen/java/package_info.go#L35C1-L108C1 documenting
// supported options.
type JavaInfo = info.Java

// PreConfigureCallback is a function to invoke prior to calling the TF provider Configure
type PreConfigureCallback = info.PreCheckCallback

// PreConfigureCallbackWithLogger is a function to invoke prior to calling the T
type PreConfigureCallbackWithLogger = info.PreConfigureCallbackWithLogger

// The types below are marshallable versions of the schema descriptions associated with a provider. These are used when
// marshalling a provider info as JSON; Note that these types only represent a subset of the information associated
// with a ProviderInfo; thus, a ProviderInfo cannot be round-tripped through JSON.

// MarshallableSchema is the JSON-marshallable form of a Terraform schema.
type MarshallableSchema = info.MarshallableSchemaShim

// MarshalSchema converts a Terraform schema into a MarshallableSchema.
func MarshalSchema(s shim.Schema) *MarshallableSchema { return info.MarshalSchemaShim(s) }

// MarshallableResource is the JSON-marshallable form of a Terraform resource schema.
type MarshallableResource = info.MarshallableResourceShim

// MarshalResource converts a Terraform resource schema into a MarshallableResource.
func MarshalResource(r shim.Resource) MarshallableResource { return info.MarshalResourceShim(r) }

// MarshallableElem is the JSON-marshallable form of a Terraform schema's element field.
type MarshallableElem = info.MarshallableElemShim

// MarshalElem converts a Terraform schema's element field into a MarshallableElem.
func MarshalElem(e interface{}) *MarshallableElem { return info.MarshalElemShim(e) }

// MarshallableProvider is the JSON-marshallable form of a Terraform provider schema.
type MarshallableProvider = info.MarshallableProviderShim

// MarshalProvider converts a Terraform provider schema into a MarshallableProvider.
func MarshalProvider(p shim.Provider) *MarshallableProvider {
	return info.MarshalProviderShim(p)
}

// MarshallableSchemaInfo is the JSON-marshallable form of a Pulumi SchemaInfo value.
type MarshallableSchemaInfo = info.MarshallableSchema

// MarshalSchemaInfo converts a Pulumi SchemaInfo value into a MarshallableSchemaInfo value.
func MarshalSchemaInfo(s *SchemaInfo) *MarshallableSchemaInfo { return info.MarshalSchema(s) }

// MarshallableDefaultInfo is the JSON-marshallable form of a Pulumi DefaultInfo value.
type MarshallableDefaultInfo = info.MarshallableDefault

// MarshalDefaultInfo converts a Pulumi DefaultInfo value into a MarshallableDefaultInfo value.
func MarshalDefaultInfo(d *DefaultInfo) *MarshallableDefaultInfo { return info.MarshalDefault(d) }

// MarshallableResourceInfo is the JSON-marshallable form of a Pulumi ResourceInfo value.
type MarshallableResourceInfo = info.MarshallableResource

// MarshalResourceInfo converts a Pulumi ResourceInfo value into a MarshallableResourceInfo value.
func MarshalResourceInfo(r *ResourceInfo) *MarshallableResourceInfo {
	return info.MarshalResource(r)
}

// MarshallableDataSourceInfo is the JSON-marshallable form of a Pulumi DataSourceInfo value.
type MarshallableDataSourceInfo = info.MarshallableDataSource

// MarshalDataSourceInfo converts a Pulumi DataSourceInfo value into a MarshallableDataSourceInfo value.
func MarshalDataSourceInfo(d *DataSourceInfo) *MarshallableDataSourceInfo {
	return info.MarshalDataSource(d)
}

// MarshallableProviderInfo is the JSON-marshallable form of a Pulumi ProviderInfo value.
type MarshallableProviderInfo = info.MarshallableProvider

// MarshalProviderInfo converts a Pulumi ProviderInfo value into a MarshallableProviderInfo value.
func MarshalProviderInfo(p *ProviderInfo) *MarshallableProviderInfo { return info.MarshalProvider(p) }

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
//
// Deprecated: Use MakeResource or call into the `tokens` module in
// "github.com/pulumi/pulumi/sdk/v3/go/common/tokens" directly.
func MakeMember(pkg string, mod string, mem string) tokens.ModuleMember {
	return tokens.ModuleMember(pkg + ":" + mod + ":" + mem)
}

// MakeType manufactures a type token for the package and the given module and type.
//
// Deprecated: Use MakeResource or call into the `tokens` module in
// "github.com/pulumi/pulumi/sdk/v3/go/common/tokens" directly.
func MakeType(pkg string, mod string, typ string) tokens.Type {
	return tokens.Type(MakeMember(pkg, mod, typ))
}

// MakeDataSource manufactures a standard Pulumi function token given a package, module, and data source name.  It
// automatically uses the main package and names the file by simply lower casing the data source's
// first character.
//
// Invalid inputs panic.
func MakeDataSource(pkg string, mod string, name string) tokens.ModuleMember {
	contract.Assertf(tokens.IsName(name), "invalid datasource name: '%s'", name)
	modT := makeModule(pkg, mod, name)
	return tokens.NewModuleMemberToken(modT, tokens.ModuleMemberName(name))
}

// makeModule manufactures a standard pulumi module from a (pkg, mod, member) triple.
//
// For example:
//
//	(pkg, mod, Resource) => pkg:mod/resource
//
// Invalid inputs panic.
func makeModule(pkg, mod, member string) tokens.Module {
	mod += "/" + string(unicode.ToLower(rune(member[0]))) + member[1:]
	contract.Assertf(tokens.IsQName(pkg), "invalid pkg name: '%s'", pkg)
	pkgT := tokens.NewPackageToken(tokens.PackageName(pkg))
	contract.Assertf(tokens.IsQName(mod), "invalid module name: '%s'", mod)
	return tokens.NewModuleToken(pkgT, tokens.ModuleName(mod))
}

// MakeResource manufactures a standard resource token given a package, module and resource name.  It
// automatically uses the main package and names the file by simply lower casing the resource's
// first character.
//
// Invalid inputs panic.
func MakeResource(pkg string, mod string, res string) tokens.Type {
	contract.Assertf(tokens.IsName(res), "invalid resource name: '%s'", res)
	modT := makeModule(pkg, mod, res)
	return tokens.NewTypeToken(modT, tokens.TypeName(res))
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

// ConfigStringValue gets a string value from a property map, then from environment vars; defaults to empty string ""
func ConfigStringValue(vars resource.PropertyMap, prop resource.PropertyKey, envs []string) string {
	val, ok := vars[prop]
	if ok && val.IsString() {
		return val.StringValue()
	}
	for _, env := range envs {
		val, ok := os.LookupEnv(env)
		if ok {
			return val
		}
	}
	return ""
}

// ConfigArrayValue takes an array value from a property map, then from environment vars; defaults to an empty array
func ConfigArrayValue(vars resource.PropertyMap, prop resource.PropertyKey, envs []string) []string {
	val, ok := vars[prop]
	var vals []string
	if ok && val.IsArray() {
		for _, v := range val.ArrayValue() {
			vals = append(vals, v.StringValue())
		}
		return vals
	}

	for _, env := range envs {
		val, ok := os.LookupEnv(env)
		if ok {
			return strings.Split(val, ";")
		}
	}
	return vals
}

// ConfigBoolValue takes a bool value from a property map, then from environment vars; defaults to false
func ConfigBoolValue(vars resource.PropertyMap, prop resource.PropertyKey, envs []string) bool {
	val, ok := vars[prop]
	if ok && val.IsBool() {
		return val.BoolValue()
	}
	for _, env := range envs {
		val, ok := os.LookupEnv(env)
		if ok && val == "true" {
			return true
		}
	}
	return false
}

// EXPERIMENTAL: the signature may change in minor releases.
type SkipExamplesArgs = info.SkipExamplesArgs

// If specified, the hook will run just prior to executing Terraform state upgrades to transform the resource state as
// stored in Pulumi. It can be used to perform idempotent corrections on corrupt state and to compensate for
// Terraform-level state upgrade not working as expected. Returns the corrected resource state and version. To be used
// with care.
//
// See also: https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/resource/schema#Schema.Version
type (
	PreStateUpgradeHook     = info.PreStateUpgradeHook
	PreStateUpgradeHookArgs = info.PreStateUpgradeHookArgs
)

func DelegateIDField(field resource.PropertyKey, providerName, repoURL string) ComputeID {
	return func(ctx context.Context, state resource.PropertyMap) (resource.ID, error) {
		err := func(msg string, a ...any) error {
			return delegateIDFieldError{
				msg:          fmt.Sprintf(msg, a...),
				providerName: providerName,
				repoURL:      repoURL,
			}
		}
		fieldValue, ok := state[field]
		if !ok {
			return "", err("Could not find required property '%s' in state", field)
		}

		contract.Assertf(
			!fieldValue.IsComputed() && (!fieldValue.IsOutput() || fieldValue.OutputValue().Known),
			"ComputeID is only called during when preview=false, so we should never need to "+
				"deal with computed properties",
		)

		if fieldValue.IsSecret() || (fieldValue.IsOutput() && fieldValue.OutputValue().Secret) {
			msg := fmt.Sprintf("Setting non-secret resource ID as '%s' (which is secret)", field)
			GetLogger(ctx).Warn(msg)
			if fieldValue.IsSecret() {
				fieldValue = fieldValue.SecretValue().Element
			} else {
				fieldValue = fieldValue.OutputValue().Element
			}
		}

		if !fieldValue.IsString() {
			return "", err("Expected '%s' property to be a string, found %s",
				field, fieldValue.TypeString())
		}

		return resource.ID(fieldValue.StringValue()), nil
	}
}

type delegateIDFieldError struct {
	msg                   string
	providerName, repoURL string
}

func (err delegateIDFieldError) Error() string {
	return fmt.Sprintf("%s. This is an error in %s resource provider, please report at %s",
		err.msg, err.providerName, err.repoURL)
}

func (err delegateIDFieldError) Is(target error) bool {
	target, ok := target.(delegateIDFieldError)
	return ok && err == target
}
