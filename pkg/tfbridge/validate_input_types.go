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
	// TODO: what if properties have defaults specified, should this still be an error?
	if len(required) > 0 {
		if len(stableKeys) == 0 {
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
			if !slices.Contains(stableKeys, resource.PropertyKey(requiredProp)) {
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
					"expected an object, got %s type",
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
		// TODO type check against union types.
		//
		// bindTypeSpecOneOf provides a good hint of how to interpret these:
		//
		// https://github.com/pulumi/pulumi/blob/master/pkg/codegen/schema/bind.go#L842
		//
		// Specifically it defines the defaultType, discriminator, mapping and elements.
		return nil
	}

	switch typeSpec.Type {
	case "boolean":
		// The bridge permits coalescing strings to booleans, hence skip strings.
		if !propertyValue.IsBool() && !propertyValue.IsString() {
			return []TypeFailure{{
				ResourcePath: propertyPath.String(),
				Reason: fmt.Sprintf(
					"expected a boolean, got %s type",
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
					"expected a number, got %s type",
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
					"expected a string, got %s type",
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
				"expected an array, got %s type",
				propertyValue.TypeString(),
			),
		}}
	case "object":
		// This is not really an object but a map type with some element type, which is assumed to be string if
		// unspecified. Check accordingly. This should be very similar to the "array" case.
		//
		// elementTypeSpec := typeSpec.AdditionalProperties
		panic("TODO")
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

// validateResourceInputType will validate a single input against the pulumi schema. It will
// return a list of type failures if any are found. This function only operates on Resources
// func (v *PulumiInputValidator) validateResourceInputType(
// 	inputName string,
// 	inputValue resource.PropertyValue,
// 	propertyPath resource.PropertyPath,
// ) *[]TypeFailure {
// 	resourceType := v.urn.Type()
// 	failures := []TypeFailure{}
// 	if resource, ok := v.schema.Resources[resourceType.String()]; ok {
// 		if prop, ok := resource.InputProperties[inputName]; ok {
// 			if failure := v.validateTypeSpec(
// 				inputName,
// 				inputValue,
// 				prop.TypeSpec,
// 				propertyPath,
// 				resource.RequiredInputs,
// 			); failure != nil {
// 				failures = append(failures, failure...)
// 			}
// 		} else {
// 			return &[]TypeFailure{
// 				{
// 					ResourcePath: propertyPath.String(),
// 					Reason:       fmt.Sprintf("property %s is not defined in the schema", inputName),
// 				},
// 			}
// 		}
// 	}

// 	if len(failures) > 0 {
// 		return &failures
// 	}

// 	return nil
// }

// type possibleTypes []string

// func (p possibleTypes) add(typ ...string) possibleTypes {
// 	for _, t := range typ {
// 		if !slices.Contains(p, t) && t != "" {
// 			p = append(p, t)

// 		}
// 	}
// 	return p
// }

// func (p possibleTypes) toString() string {
// 	return strings.Join(p, " OR ")
// }

// findOneOf attempts to find the correct type spec for a given input input value
// from a list of possible types.
// For example, given the following schema:
//
//	"prop": {
//		TypeSpec: pschema.TypeSpec{
//			OneOf: []pschema.TypeSpec{
//				{
//					Type: "array",
//					Items: &pschema.TypeSpec{
//						Type: "object",
//						// using ref to test specific object keys
//						Ref: "#/types/pkg:index/type:ObjectStringType",
//					},
//				},
//				{
//					Type: "string",
//				},
//			},
//		},
//	}
//
// and the following input value:
//
//	"prop": []map[string]interface{}{{"key": "value"}}
//
// In this case the actual type that we want to compare "prop" to is the `ObjectStringType`
// so this function will return the `Items` `TypeSpec` for comparison.
// func (v *PulumiInputValidator) validateOneOf(
// 	propertyKey string,
// 	inputValue resource.PropertyValue,
// 	specs []pschema.TypeSpec,
// 	propertyPath resource.PropertyPath,
// ) []TypeFailure {

// 	failures := []TypeFailure{}
// 	for _, spec := range specs {
// 		failure := v.validateTypeSpec(propertyKey, inputValue, spec, propertyPath, []string{})
// 		if failure == nil {
// 			return nil
// 		}
// 		failures = append(failures, failure...)
// 	}
// 	return failures
// }

// The input type (i.e. PropertyValue.TypeString() do not match 100% to the pschema.TypeSpec.Type)
// func typeEqual(schemaType string, inputType resource.PropertyValue) bool {
// 	propertyValueType := inputType.TypeString()
// 	if inputType.IsOutput() || inputType.IsSecret() {
// 		value := inputType.TypeString()
// 		// regex should match `secret<string>` or `output<string>`
// 		r := regexp.MustCompile(`(?:secret<|output<)+(\w+)>+`)
// 		matches := r.FindAllStringSubmatch(value, -1)
// 		propertyValueType = matches[0][1]
// 	}

// 	if schemaType == propertyValueType {
// 		return true
// 	}
// 	if schemaType == "array" && inputType.IsArray() {
// 		return true
// 	}

// 	// When we convert types to the terraform type we automatically convert numbers to strings
// 	if (schemaType == "number" || schemaType == "integer" || schemaType == "string") && inputType.IsNumber() {
// 		return true

// 	}
// 	// When we convert types to the terraform type we automatically convert bools to strings
// 	if (schemaType == "boolean" || schemaType == "string") && inputType.IsBool() {
// 		return true
// 	}

// 	return false
// }

// matchType performs some initial type matching for a given input type.
// returns the possible types if a type is not matched
// func (v *PulumiInputValidator) matchType(
// 	inputType resource.PropertyValue,
// 	specs ...pschema.TypeSpec,
// ) (possibleTypes, bool) {
// 	possibleTypes := possibleTypes{}

// 	if len(specs) == 0 {
// 		return possibleTypes, false
// 	}

// 	for _, spec := range specs {
// 		possibleTypes = possibleTypes.add(spec.Type)
// 		// if the type matches then we are done
// 		if typeEqual(spec.Type, inputType) {
// 			return []string{}, true
// 		}

// 		// if the type is a ref, we need to actually check the ref type
// 		// the only edge case is if the type is an enum. For enums the input type would
// 		// be a string, but an enum type is an object
// 		if spec.Ref != "" {
// 			refType := v.getType(spec.Ref)
// 			if refType != nil {
// 				possibleTypes = possibleTypes.add(refType.Type)
// 				if len(refType.Enum) > 0 {
// 					if inputType.IsString() {
// 						return []string{}, true
// 					}
// 				}
// 				if typeEqual(refType.Type, inputType) {
// 					return []string{}, true
// 				}
// 			}
// 		}

// 		if spec.AdditionalProperties != nil && len(spec.AdditionalProperties.OneOf) > 0 {
// 			for _, oneOfSpec := range spec.AdditionalProperties.OneOf {
// 				possibleTypes = possibleTypes.add(oneOfSpec.Type)
// 				if typeEqual(oneOfSpec.Type, inputType) {
// 					return []string{}, true
// 				}
// 			}
// 		}

// 		// If the type is an array, we need to check the items
// 		if len(spec.OneOf) > 0 {
// 			possible, ok := v.matchType(inputType, spec.OneOf...)
// 			if ok {
// 				return []string{}, true
// 			}
// 			possibleTypes = possibleTypes.add(possible...)
// 		}
// 	}

// 	return possibleTypes, false
// }
