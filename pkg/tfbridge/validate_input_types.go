package tfbridge

import (
	"fmt"
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type PulumiInputValidator struct {
	// The resource URN that we are validating
	urn resource.URN

	// The pulumi schema of the package
	schema pschema.PackageSpec

	// Whether or not to fail when properties are provided
	// that do not exist in the spec. Defaults to false
	validateUnknownTypes bool
}

type TypeFailure struct {
	// The Reason for the type failure
	Reason string

	// The path to the property that failed
	ResourcePath string
}

// NewInputValidator creates a new input validator for a given resource and
// package schema
func NewInputValidator(urn resource.URN, schema pschema.PackageSpec, validateUnknownTypes bool) *PulumiInputValidator {
	return &PulumiInputValidator{
		urn:                  urn,
		schema:               schema,
		validateUnknownTypes: validateUnknownTypes,
	}
}

// validatePropertyValue is the main function for validating a PropertyMap against the Pulumi Schema. It is a
// recursive function that will validate nested types and arrays. Returns a list of type failures if any are found.
func (v *PulumiInputValidator) validatePropertyMap(
	propertyMap resource.PropertyMap,
	propertyTypes map[string]pschema.PropertySpec,
	propertyPath resource.PropertyPath,
) []TypeFailure {
	stableKeys := propertyMap.StableKeys()
	failures := []TypeFailure{}

	// TODO[pulumi/pulumi-terrafor-bridge#1892]: handle required properties. Deferring for now
	// because properties can be filled in later and we don't want to
	// fail too aggressively
	for _, objectKey := range stableKeys {
		objectValue := propertyMap[objectKey]
		objType, knownType := propertyTypes[string(objectKey)]
		if !knownType {
			if v.validateUnknownTypes {
				failures = append(failures, TypeFailure{
					Reason:       fmt.Sprintf("an unexpected argument %q was provided", string(objectKey)),
					ResourcePath: propertyPath.String(),
				})
			}
			// permit extraneous properties to flow through
			continue
		}
		subPropertyPath := append(propertyPath, string(objectKey))
		failure := v.validatePropertyValue(objectValue, objType.TypeSpec, subPropertyPath)
		if failure != nil {
			failures = append(failures, failure...)
		}
	}
	return failures
}

// validatePropertyValue is the main function for validating a PropertyValue against the Pulumi Schema. It is a
// recursive function that will validate nested types and arrays. Returns a list of type failures if any are found.
func (v *PulumiInputValidator) validatePropertyValue(
	propertyValue resource.PropertyValue,
	typeSpec pschema.TypeSpec,
	propertyPath resource.PropertyPath,
) []TypeFailure {
	// don't type check
	// - resource references (not yet)
	// - assets (not yet)
	// - archives (not yet)
	// - computed values (they are allowed for any type)
	// - unknown output values (similar to computed)
	// - nulls (they are allowed for any type, and for missing required properties see validatePropertyMap)
	if propertyValue.IsResourceReference() ||
		propertyValue.IsAsset() ||
		propertyValue.IsArchive() ||
		propertyValue.IsComputed() ||
		propertyValue.IsNull() ||
		(propertyValue.IsOutput() && !propertyValue.OutputValue().Known) {
		return nil
	}

	// for known first-class outputs simply validate their known value
	if propertyValue.IsOutput() && propertyValue.OutputValue().Known {
		elementValue := propertyValue.OutputValue().Element
		return v.validatePropertyValue(elementValue, typeSpec, propertyPath)
	}

	// for secrets validate their inner value
	if propertyValue.IsSecret() {
		elementValue := propertyValue.SecretValue().Element
		return v.validatePropertyValue(elementValue, typeSpec, propertyPath)
	}

	// now we are going to switch on the desired type
	//
	// a good reference here for the semantics of the TypeSpec is
	// https://github.com/pulumi/pulumi/blob/master/pkg/codegen/schema/bind.go#L881
	//
	// this code is not using the binder functionality as that would require a plugin loader to resolve references,
	// which is not desirable for the provider runtime
	//
	// possibly this could be done in the future with a way to still use the bind code while resolving references to
	// unchecked types as using the bound representation would make the code cleaner
	//
	// for now the code simply follows bindSpec

	if typeSpec.Ref != "" {
		objType := v.getType(typeSpec.Ref)
		// Refusing to validate unknown or unresolved type.
		if objType == nil {
			return nil
		}

		if !propertyValue.IsObject() {
			return []TypeFailure{{
				ResourcePath: propertyPath.String(),
				Reason: fmt.Sprintf(
					"expected object type, got %s type",
					propertyValue.TypeString(),
				),
			}}
		}

		return v.validatePropertyMap(
			propertyValue.ObjectValue(),
			objType.Properties,
			propertyPath,
		)
	}

	if typeSpec.OneOf != nil {
		// TODO[pulumi/pulumi-terrafor-bridge#1891]: handle OneOf types
		// bindTypeSpecOneOf provides a good hint of how to interpret these:
		//
		// https://github.com/pulumi/pulumi/blob/master/pkg/codegen/schema/bind.go#L842
		//
		// Specifically it defines the defaultType, discriminator, mapping and elements.
		return nil
	}

	switch typeSpec.Type {
	case "boolean":
		if !propertyValue.IsBool() {
			return []TypeFailure{{
				ResourcePath: propertyPath.String(),
				Reason: fmt.Sprintf(
					"expected boolean type, got %s type",
					propertyValue.TypeString(),
				),
			}}
		}
		return nil
	case "integer", "number":
		// The bridge permits coalescing strings to numbers, hence skip strings.
		if !propertyValue.IsNumber() && !propertyValue.IsString() {
			return []TypeFailure{{
				ResourcePath: propertyPath.String(),
				Reason: fmt.Sprintf(
					"expected number type, got %s type",
					propertyValue.TypeString(),
				),
			}}
		}
		return nil
	case "string":
		// The bridge permits coalescing numbers and booleans to strings, hence skip these.
		if !propertyValue.IsString() && !propertyValue.IsNumber() && !propertyValue.IsBool() {
			return []TypeFailure{{
				ResourcePath: propertyPath.String(),
				Reason: fmt.Sprintf(
					"expected string type, got %s type",
					propertyValue.TypeString(),
				),
			}}
		}
		return nil
	case "array":
		if propertyValue.IsArray() {
			if typeSpec.Items == nil {
				// Unknown item type so nothing more to check.
				return nil
			}
			// Check every item against the array element type.
			failures := []TypeFailure{}
			for idx, arrayValue := range propertyValue.ArrayValue() {
				pb := append(propertyPath, idx)
				failure := v.validatePropertyValue(arrayValue, *typeSpec.Items, pb)
				if failure != nil {
					failures = append(failures, failure...)
				}
			}
			return failures
		}
		return []TypeFailure{{
			ResourcePath: propertyPath.String(),
			Reason: fmt.Sprintf(
				"expected array type, got %s type",
				propertyValue.TypeString(),
			),
		}}
	case "object":
		// This is not really an object but a map type with some element type, which is assumed to be string if
		// unspecified. Check accordingly. This should be very similar to the "array" case.
		//
		// elementTypeSpec := typeSpec.AdditionalProperties
		if propertyValue.IsObject() {
			if typeSpec.AdditionalProperties == nil {
				// Unknown item type so nothing more to check
				return nil
			}
			objectValue := propertyValue.ObjectValue()
			failures := []TypeFailure{}
			for _, propertyKey := range objectValue.StableKeys() {
				if strings.HasPrefix(string(propertyKey), "__") {
					continue
				}
				pb := append(propertyPath, string(propertyKey))
				failure := v.validatePropertyValue(objectValue[propertyKey], *typeSpec.AdditionalProperties, pb)
				if failure != nil {
					failures = append(failures, failure...)
				}
			}
			return failures
		}
		return []TypeFailure{{
			ResourcePath: propertyPath.String(),
			Reason: fmt.Sprintf(
				"expected object type, got %s type",
				propertyValue.TypeString(),
			),
		}}
	default:
		// Unrecognized type, assume no errors.
		return nil
	}
}

// getType gets a type definition from a schema reference. Currently it only supports types from the same schema that
// are object types. It does not support enum types, foreign type references, special references such as
// "pulumi.json#/Archive", references to resources or providers or anything else.
func (v *PulumiInputValidator) getType(typeRef string) *pschema.ObjectTypeSpec {
	if strings.HasPrefix(typeRef, "#/types/") {
		ref := strings.TrimPrefix(typeRef, "#/types/")
		if typeSpec, ok := v.schema.Types[ref]; ok {
			// Exclude enum types here.
			if len(typeSpec.Enum) == 0 {
				return &typeSpec.ObjectTypeSpec
			}
		}
	}
	return nil
}

// ValidateInputs will validate a set of inputs against the pulumi schema. It will
// return a list of type failures if any are found
func (v *PulumiInputValidator) ValidateInputs(resourceToken tokens.Type, inputs resource.PropertyMap) []TypeFailure {
	resourceSpec, knownResourceSpec := v.schema.Resources[string(resourceToken)]
	if !knownResourceSpec {
		return nil
	}
	return v.validatePropertyMap(
		inputs,
		resourceSpec.InputProperties,
		resource.PropertyPath{},
	)
}

// ValidateConfig will validate the provider config against the pulumi schema. It will
// return a list of type failures if any are found
func (v *PulumiInputValidator) ValidateConfig(inputs resource.PropertyMap) []TypeFailure {
	// The inputs that are provided to `CheckConfig` also include pulumi options
	// so make sure we are only checking against properties in the config
	inputsToValidate := resource.PropertyMap{}
	for _, k := range inputs.StableKeys() {
		if _, ok := v.schema.Config.Variables[string(k)]; ok {
			inputsToValidate[k] = inputs[k]
		}
	}
	return v.validatePropertyMap(
		inputsToValidate,
		v.schema.Config.Variables,
		resource.PropertyPath{},
	)
}
