package tfbridge

import (
	"fmt"
	"slices"
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
}

type TypeFailure struct {
	// The Reason for the type failure
	Reason string

	// The path to the property that failed
	ResourcePath string
}

// NewInputValidator creates a new input validator for a given resource and
// package schema
func NewInputValidator(urn resource.URN, schema pschema.PackageSpec) *PulumiInputValidator {
	return &PulumiInputValidator{
		urn:    urn,
		schema: schema,
	}
}

func propertiesWithDefaults(
	propertyTypes map[string]pschema.PropertySpec,
) []resource.PropertyKey {
	keysWithDefaults := []resource.PropertyKey{}
	for key, spec := range propertyTypes {
		if spec.Default != nil {
			keysWithDefaults = append(keysWithDefaults, resource.PropertyKey(key))

		}
	}
	return keysWithDefaults
}

// validatePropertyValue is the main function for validating a PropertyMap against the Pulumi Schema. It is a
// recursive function that will validate nested types and arrays. Returns a list of type failures if any are found.
func (v *PulumiInputValidator) validatePropertyMap(
	propertyMap resource.PropertyMap,
	required []string,
	propertyTypes map[string]pschema.PropertySpec,
	propertyPath resource.PropertyPath,
) []TypeFailure {
	stableKeys := propertyMap.StableKeys()
	failures := []TypeFailure{}

	// handle required properties
	//
	// add properties that have defaults to the list since those will have
	// their values populated later and we do not want to error now if they
	// are missing
	keysAndDefaults := append(stableKeys, propertiesWithDefaults(propertyTypes)...)
	if len(required) > 0 {
		if len(keysAndDefaults) == 0 {
			failures = append(failures, TypeFailure{
				ResourcePath: propertyPath.String(),
				Reason: fmt.Sprintf(
					"%s is missing required properties: %s",
					propertyPath.String(), strings.Join(required, ", "),
				),
			})
			return failures
		}
		for _, requiredProp := range required {
			if !slices.Contains(keysAndDefaults, resource.PropertyKey(requiredProp)) {
				failures = append(failures, TypeFailure{
					ResourcePath: propertyPath.String(),
					Reason: fmt.Sprintf(
						"%s is missing a required property: %s",
						propertyPath.String(), requiredProp,
					),
				})
			}
		}
		if len(failures) > 0 {
			return failures
		}
	}

	for _, objectKey := range stableKeys {
		objectValue := propertyMap[objectKey]
		objType, knownType := propertyTypes[string(objectKey)]
		if !knownType {
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
			objType.Required,
			objType.Properties,
			propertyPath,
		)
	}

	if typeSpec.OneOf != nil {
		// bindTypeSpecOneOf provides a good hint of how to interpret these:
		//
		// https://github.com/pulumi/pulumi/blob/master/pkg/codegen/schema/bind.go#L842
		//
		// Specifically it defines the defaultType, discriminator, mapping and elements.
		if len(typeSpec.OneOf) < 2 {
			// invalid type, don't validate
			return nil
		}

		// don't validate types with Discriminator (yet)
		if typeSpec.Discriminator != nil {
			return nil
		}

		// default type
		if typeSpec.Type != "" {
			typeSpec.OneOf = append(typeSpec.OneOf, pschema.TypeSpec{Type: typeSpec.Type})
		}

		failures := []TypeFailure{}
		for _, spec := range typeSpec.OneOf {
			failure := v.validatePropertyValue(propertyValue, spec, propertyPath)
			if len(failure) == 0 {
				// we have found the correct type
				return nil
			}
			failures = append(failures, failure...)
		}
		return failures
	}

	switch typeSpec.Type {
	case "bool":
		// The bridge permits coalescing strings to booleans, hence skip strings.
		if !propertyValue.IsBool() && !propertyValue.IsString() {
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
			// if there is a ref to another type, get that and then validate
			if typeSpec.AdditionalProperties.Ref != "" {
				objType := v.getType(typeSpec.AdditionalProperties.Ref)
				// Refusing to validate unknown or unresolved type.
				if objType == nil {
					return nil
				}

				return v.validatePropertyMap(
					propertyValue.ObjectValue(),
					objType.Required,
					objType.Properties,
					propertyPath,
				)
			}
			objectValue := propertyValue.ObjectValue()
			failures := []TypeFailure{}
			for _, propertyKey := range objectValue.StableKeys() {
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
func (v *PulumiInputValidator) ValidateInputs(resourceToken tokens.Type, inputs resource.PropertyMap) *[]TypeFailure {
	resourceSpec, knownResourceSpec := v.schema.Resources[string(resourceToken)]
	if !knownResourceSpec {
		return nil
	}
	failures := v.validatePropertyMap(
		inputs,
		resourceSpec.RequiredInputs,
		resourceSpec.InputProperties,
		resource.PropertyPath{},
	)
	if len(failures) > 0 {
		return &failures
	}

	return nil
}
