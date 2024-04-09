package tfbridge

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// pathBuilder is a helper for building property paths for
// better error messages. As you recurse through objects and arrays
// you can build up a path to the property that failed
type pathBuilder struct {
	paths  []string
	isList bool
}

// add adds a property path to the path builder
func (c *pathBuilder) add(path string) {
	if !c.isList {
		c.paths = append(c.paths, path)
	}
	c.isList = false
}

// addListIndex adds a list index to the path builder, but does not mutate the
// original path builder. This allows for subsequent list items to have a
// distinct path builder
func (c *pathBuilder) addListIndex(idx int) pathBuilder {
	cc := *c
	cc.paths = append(c.paths, fmt.Sprintf("%d", idx))
	cc.isList = true
	return cc
}

// toPath renders the property path as a string
func (c *pathBuilder) toPath() string {
	return strings.Join(c.paths, ".")
}

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

// getObjectTypeSpec gets a type spec for an object type. Objects can have their
// type defined in a couple different ways. This will check the different ways
// and return the correct type spec.
func (v *PulumiInputValidator) getObjectTypeSpec(
	spec *pschema.TypeSpec,
	propertyValue resource.PropertyValue,
) (
	complexSpec *pschema.ComplexTypeSpec,
	typeSpec *pschema.TypeSpec,
	requiredProps []string,
) {

	if spec.AdditionalProperties != nil {
		if spec.AdditionalProperties.Ref != "" {
			complexSpec = v.getType(spec.AdditionalProperties.Ref)
			// if we don't find the reference then just continue instead of failing
			if complexSpec != nil {
				requiredProps = complexSpec.Required
			}
		} else {
			typeSpec = spec.AdditionalProperties
		}
	} else if spec.Ref != "" {
		complexSpec = v.getType(spec.Ref)
		if complexSpec != nil {
			requiredProps = complexSpec.Required
		}
	}
	return complexSpec, typeSpec, requiredProps
}

// validateTypeSpec is the main function for validating a PropertyValue against the pulumi schema.
// It is a recursive function that will validate nested types and arrays. It will return a list of
// type failures if any are found
func (v *PulumiInputValidator) validateTypeSpec(
	propertyKey string,
	propertyValue resource.PropertyValue,
	typeSpec pschema.TypeSpec,
	pathBuilder pathBuilder,
	requiredProps []string,
) []TypeFailure {
	// for now don't validate discriminators
	if typeSpec.Discriminator != nil {
		return nil
	}
	// don't type check
	// - resource references
	// - assets
	// - archives
	// - computed values
	if propertyValue.IsResourceReference() ||
		propertyValue.IsAsset() ||
		propertyValue.IsArchive() ||
		propertyValue.IsComputed() {
		return nil
	}

	// handle non-required properties that are null
	if propertyValue.IsNull() {
		if slices.Contains(requiredProps, propertyKey) {
			return []TypeFailure{
				{
					ResourcePath: pathBuilder.toPath(),
					Reason:       fmt.Sprintf("property %s is required", propertyKey),
				},
			}
		}
	}

	failures := []TypeFailure{}
	pathBuilder.add(propertyKey)

	// perform some initial easy type matching. fail fast.
	possible, ok := v.matchType(propertyValue, typeSpec)
	if !ok {
		return []TypeFailure{
			{
				ResourcePath: pathBuilder.toPath(),
				Reason: fmt.Sprintf(
					"expected %s type, got %s type",
					possible.toString(),
					propertyValue.TypeString(),
				),
			},
		}
	}

	// array type
	if propertyValue.IsArray() {
		for idx, arrayValue := range propertyValue.ArrayValue() {
			pb := pathBuilder.addListIndex(idx)
			if typeSpec.Items != nil {
				failure := v.validateTypeSpec(propertyKey, arrayValue, *typeSpec.Items, pb, requiredProps)
				if failure != nil {
					failures = append(failures, failure...)
				}
			} else if len(typeSpec.OneOf) > 0 {
				failure := v.validateOneOf(propertyKey, arrayValue, typeSpec.OneOf, pb)
				if failure != nil {
					failures = append(failures, failure...)
				}
			}
		}
	}
	if propertyValue.IsObject() {
		complexSpec, objectTypeSpec, requiredObjectProps := v.getObjectTypeSpec(&typeSpec, propertyValue)
		propObjectValue := propertyValue.ObjectValue()
		stableKeys := propObjectValue.StableKeys()

		// handle required properties
		if len(requiredObjectProps) > 0 {
			if len(stableKeys) == 0 {
				failures = append(failures, TypeFailure{
					ResourcePath: pathBuilder.toPath(),
					Reason: fmt.Sprintf(
						"%s object is missing required properties: %s",
						propertyKey, strings.Join(requiredObjectProps, ", "),
					),
				})
				return failures
			}
			for _, requiredProp := range requiredObjectProps {
				if !slices.Contains(stableKeys, resource.PropertyKey(requiredProp)) {
					failures = append(failures, TypeFailure{
						ResourcePath: pathBuilder.toPath(),
						Reason: fmt.Sprintf(
							"%s object is missing required property: %s",
							propertyKey, requiredProp,
						),
					})
				}
			}
			if len(failures) > 0 {
				return failures
			}
		}

		for _, objectKey := range stableKeys {
			objectValue := propObjectValue[objectKey]
			if complexSpec != nil {
				complexProps, ok := complexSpec.Properties[string(objectKey)]
				if !ok {
					failures = append(failures, TypeFailure{
						ResourcePath: pathBuilder.toPath(),
						Reason: fmt.Sprintf(
							"property %s is not defined in the schema",
							string(objectKey),
						),
					})
					continue
				}
				objectTypeSpec = &complexProps.TypeSpec
			}
			if objectTypeSpec != nil {
				if len(objectTypeSpec.OneOf) > 0 {
					if failure := v.validateOneOf(string(objectKey), objectValue, objectTypeSpec.OneOf, pathBuilder); failure != nil {
						failures = append(failures, failure...)
					}
					continue
				}
				if failure := v.validateTypeSpec(
					string(objectKey),
					objectValue,
					*objectTypeSpec,
					pathBuilder,
					requiredObjectProps,
				); failure != nil {
					failures = append(failures, failure...)
				}
			}
		}
	}
	// The number and string cases are for enums. The normal cases are handled
	// in the matchType function right now we are not validating enums. Enums
	// are more of a helper, but are not exhaustive in all cases i.e. ec2
	// instance types
	// if propertyValue.IsNumber() || propertyValue.IsString() {}

	if len(failures) > 0 {
		return failures
	}

	return nil
}

// getType gets a type definition from a schema reference. Currently it only
// supports types from the same schema
func (v *PulumiInputValidator) getType(typeRef string) *pschema.ComplexTypeSpec {
	if strings.HasPrefix(typeRef, "#/types/") {
		ref := strings.TrimPrefix(typeRef, "#/types/")
		if typeSpec, ok := v.schema.Types[ref]; ok {
			return &typeSpec
		}
	}
	return nil
}

// ValidateInputs will validate a set of inputs against the pulumi schema. It will
// return a list of type failures if any are found
func (v *PulumiInputValidator) ValidateInputs(inputs resource.PropertyMap) *[]TypeFailure {
	failures := []TypeFailure{}
	for key, value := range inputs {
		if failure := v.validateResourceInputType(string(key), value, pathBuilder{paths: []string{}}); failure != nil {
			failures = append(failures, *failure...)
		}
	}

	if len(failures) > 0 {
		return &failures
	}

	return nil
}

// validateResourceInputType will validate a single input against the pulumi schema. It will
// return a list of type failures if any are found. This function only operates on Resources
func (v *PulumiInputValidator) validateResourceInputType(
	inputName string,
	inputValue resource.PropertyValue,
	pathBuilder pathBuilder,
) *[]TypeFailure {
	resourceType := v.urn.Type()
	failures := []TypeFailure{}
	if resource, ok := v.schema.Resources[resourceType.String()]; ok {
		if prop, ok := resource.InputProperties[inputName]; ok {
			if failure := v.validateTypeSpec(
				inputName,
				inputValue,
				prop.TypeSpec,
				pathBuilder,
				resource.RequiredInputs,
			); failure != nil {
				failures = append(failures, failure...)
			}
		} else {
			return &[]TypeFailure{
				{
					ResourcePath: pathBuilder.toPath(),
					Reason:       fmt.Sprintf("property %s is not defined in the schema", inputName),
				},
			}
		}
	}

	if len(failures) > 0 {
		return &failures
	}

	return nil
}

type possibleTypes []string

func (p possibleTypes) add(typ ...string) possibleTypes {
	for _, t := range typ {
		if !slices.Contains(p, t) && t != "" {
			p = append(p, t)

		}
	}
	return p
}

func (p possibleTypes) toString() string {
	return strings.Join(p, " OR ")
}

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
func (v *PulumiInputValidator) validateOneOf(
	propertyKey string,
	inputValue resource.PropertyValue,
	specs []pschema.TypeSpec,
	pathBuilder pathBuilder,
) []TypeFailure {

	failures := []TypeFailure{}
	for _, spec := range specs {
		failure := v.validateTypeSpec(propertyKey, inputValue, spec, pathBuilder, []string{})
		if failure == nil {
			return nil
		}
		failures = append(failures, failure...)
	}
	return failures
}

// The input type (i.e. PropertyValue.TypeString() do not match 100% to the pschema.TypeSpec.Type)
func typeEqual(schemaType string, inputType resource.PropertyValue) bool {
	propertyValueType := inputType.TypeString()
	if inputType.IsOutput() || inputType.IsSecret() {
		value := inputType.TypeString()
		// regex should match `secret<string>` or `output<string>`
		r := regexp.MustCompile(`(?:secret<|output<)+(\w+)>+`)
		matches := r.FindAllStringSubmatch(value, -1)
		propertyValueType = matches[0][1]
	}

	if schemaType == propertyValueType {
		return true
	}
	if schemaType == "array" && inputType.IsArray() {
		return true
	}

	// When we convert types to the terraform type we automatically convert numbers to strings
	if (schemaType == "number" || schemaType == "integer" || schemaType == "string") && inputType.IsNumber() {
		return true

	}
	// When we convert types to the terraform type we automatically convert bools to strings
	if (schemaType == "boolean" || schemaType == "string") && inputType.IsBool() {
		return true
	}

	return false
}

// matchType performs some initial type matching for a given input type.
// returns the possible types if a type is not matched
func (v *PulumiInputValidator) matchType(
	inputType resource.PropertyValue,
	specs ...pschema.TypeSpec,
) (possibleTypes, bool) {
	possibleTypes := possibleTypes{}

	if len(specs) == 0 {
		return possibleTypes, false
	}

	for _, spec := range specs {
		possibleTypes = possibleTypes.add(spec.Type)
		// if the type matches then we are done
		if typeEqual(spec.Type, inputType) {
			return []string{}, true
		}

		// if the type is a ref, we need to actually check the ref type
		// the only edge case is if the type is an enum. For enums the input type would
		// be a string, but an enum type is an object
		if spec.Ref != "" {
			refType := v.getType(spec.Ref)
			if refType != nil {
				possibleTypes = possibleTypes.add(refType.Type)
				if len(refType.Enum) > 0 {
					if inputType.IsString() {
						return []string{}, true
					}
				}
				if typeEqual(refType.Type, inputType) {
					return []string{}, true
				}
			}
		}

		if spec.AdditionalProperties != nil && len(spec.AdditionalProperties.OneOf) > 0 {
			for _, oneOfSpec := range spec.AdditionalProperties.OneOf {
				possibleTypes = possibleTypes.add(oneOfSpec.Type)
				if typeEqual(oneOfSpec.Type, inputType) {
					return []string{}, true
				}
			}
		}

		// If the type is an array, we need to check the items
		if len(spec.OneOf) > 0 {
			possible, ok := v.matchType(inputType, spec.OneOf...)
			if ok {
				return []string{}, true
			}
			possibleTypes = possibleTypes.add(possible...)
		}
	}

	return possibleTypes, false
}
