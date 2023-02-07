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

package convert

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type encoding struct {
	spec          PackageSpec
	propertyNames PropertyNames
}

var _ Encoding = (*encoding)(nil)

func NewEncoding(spec PackageSpec, propertyNames PropertyNames) Encoding {
	return &encoding{spec: spec, propertyNames: propertyNames}
}

func (e *encoding) NewConfigEncoder(configType tftypes.Object) (Encoder, error) {
	spec := specFinder(e.spec.Config().Variables)
	propNames := NewConfigPropertyNames(e.propertyNames)
	propertyEncoders, err := e.buildPropertyEncoders(propNames, spec, configType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for provider config: %w", err)
	}
	enc, err := newObjectEncoder(configType, propertyEncoders, propNames)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for provider config: %w", err)
	}
	return enc, nil
}

func (e *encoding) NewResourceEncoder(resourceToken tokens.Type, objectType tftypes.Object) (Encoder, error) {
	rspec := e.spec.Resource(resourceToken)
	if rspec == nil {
		return nil, fmt.Errorf("dangling resource token %q", string(resourceToken))
	}
	spec := specFinderWithID(rspec.Properties)
	propNames := NewTypeLocalPropertyNames(e.propertyNames, tokens.Token(resourceToken))
	propertyEncoders, err := e.buildPropertyEncoders(propNames, spec, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for resource %q: %w", string(resourceToken), err)
	}
	enc, err := newObjectEncoder(objectType, propertyEncoders, propNames)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for resource %q: %w", string(resourceToken), err)
	}
	return enc, nil
}

func (e *encoding) NewResourceDecoder(resourceToken tokens.Type, objectType tftypes.Object) (Decoder, error) {
	rspec := e.spec.Resource(resourceToken)
	if rspec == nil {
		return nil, fmt.Errorf("dangling resource token %q", string(resourceToken))
	}
	spec := specFinderWithID(rspec.Properties)
	propNames := NewTypeLocalPropertyNames(e.propertyNames, tokens.Token(resourceToken))
	propertyDecoders, err := e.buildPropertyDecoders(propNames, spec, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an decoder for resource %q: %w", string(resourceToken), err)
	}
	propertyDecoders["id"] = newStringDecoder()
	dec, err := newObjectDecoder(objectType, propertyDecoders, propNames)
	if err != nil {
		return nil, fmt.Errorf("cannot derive a decoder for resource %q: %w", string(resourceToken), err)
	}
	return dec, nil
}

func (e *encoding) NewDataSourceEncoder(functionToken tokens.ModuleMember, objectType tftypes.Object) (Encoder, error) {
	fspec := e.spec.Function(functionToken)
	if fspec == nil {
		return nil, fmt.Errorf("dangling function token %q", string(functionToken))
	}
	token := tokens.Token(functionToken)
	spec := specFinderWithFallback(specFinder(fspec.Inputs.Properties), specFinder(funcOutputs(fspec).Properties))
	propNames := NewTypeLocalPropertyNames(e.propertyNames, token)
	propertyEncoders, err := e.buildPropertyEncoders(propNames, spec, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for function %q: %w", string(token), err)
	}
	enc, err := newObjectEncoder(objectType, propertyEncoders, propNames)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for function %q: %w", string(token), err)
	}
	return enc, nil
}

func (e *encoding) NewDataSourceDecoder(functionToken tokens.ModuleMember, objectType tftypes.Object) (Decoder, error) {
	token := tokens.Token(functionToken)
	fspec := e.spec.Function(functionToken)
	if fspec == nil {
		return nil, fmt.Errorf("dangling function token %q", string(token))
	}
	spec := specFinderWithFallback(specFinder(funcOutputs(fspec).Properties), specFinder(fspec.Inputs.Properties))
	propNames := NewTypeLocalPropertyNames(e.propertyNames, token)
	propertyDecoders, err := e.buildPropertyDecoders(propNames, spec, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an decoder for function %q: %w", string(token), err)
	}
	dec, err := newObjectDecoder(objectType, propertyDecoders, propNames)
	if err != nil {
		return nil, fmt.Errorf("cannot derive a decoder for function %q: %w", string(token), err)
	}
	return dec, nil
}

func (e *encoding) buildPropertyEncoders(
	propertyNames LocalPropertyNames,
	propSpecs func(resource.PropertyKey) *pschema.PropertySpec,
	objectType tftypes.Object,
) (map[TerraformPropertyName]Encoder, error) {
	propertyEncoders := map[TerraformPropertyName]Encoder{}
	for tfName, t := range objectType.AttributeTypes {
		key := propertyNames.PropertyKey(tfName, t)
		puSpec := propSpecs(key)
		if puSpec == nil {
			return nil, fmt.Errorf("missing property %q", string(key))
		}
		enc, err := e.newPropertyEncoder(tfName, *puSpec, t)
		if err != nil {
			return nil, err
		}
		propertyEncoders[tfName] = enc
	}
	return propertyEncoders, nil
}

func (e *encoding) buildPropertyDecoders(
	propertyNames LocalPropertyNames,
	propSpecs func(resource.PropertyKey) *pschema.PropertySpec,
	objectType tftypes.Object,
) (map[TerraformPropertyName]Decoder, error) {
	propertyEncoders := map[TerraformPropertyName]Decoder{}
	for tfName, t := range objectType.AttributeTypes {
		key := propertyNames.PropertyKey(tfName, t)
		puSpec := propSpecs(key)
		if puSpec == nil {
			return nil, fmt.Errorf("missing property %q", string(key))
		}
		dec, err := e.newPropertyDecoder(tfName, *puSpec, t)
		if err != nil {
			return nil, err
		}
		propertyEncoders[tfName] = dec
	}
	return propertyEncoders, nil
}

func (e *encoding) newPropertyEncoder(name string, propSpec pschema.PropertySpec, t tftypes.Type) (Encoder, error) {
	enc, err := e.deriveEncoder(&propSpec.TypeSpec, t)
	if err != nil {
		return nil, fmt.Errorf("Cannot derive an encoder for property %q: %w", name, err)
	}

	if propSpec.Secret {
		return newSecretEncoder(enc, t)
	}

	return enc, nil
}

func (e *encoding) newPropertyDecoder(name string, propSpec pschema.PropertySpec, t tftypes.Type) (Decoder, error) {
	dec, err := e.deriveDecoder(&propSpec.TypeSpec, t)
	if err != nil {
		return nil, fmt.Errorf("Cannot derive a decoder for property %q: %w", name, err)
	}
	if propSpec.Secret {
		return newSecretDecoder(dec)
	}
	return dec, nil
}

func (e *encoding) resolveRef(ref string) (tokens.Token, *pschema.ComplexTypeSpec, error) {
	tok := tokens.Type(strings.TrimPrefix(ref, "#/types/"))
	refSpec := e.spec.Type(tok)
	if refSpec == nil {
		return "", nil, fmt.Errorf("dangling schema ref: %q", ref)
	}
	return tokens.Token(tok), refSpec, nil
}

func (e *encoding) deriveEncoderForNamedObjectType(ref string, t tftypes.Object) (Encoder, error) {
	tok, refSpec, err := e.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	if refSpec.Enum != nil {
		return nil, fmt.Errorf("enums are not supported: %q", ref)
	}
	propNames := NewTypeLocalPropertyNames(e.propertyNames, tok)
	propertyEncoders, err := e.buildPropertyEncoders(propNames, specFinder(refSpec.Properties), t)
	if err != nil {
		return nil, fmt.Errorf("issue deriving an encoder for %q: %w", ref, err)
	}
	return newObjectEncoder(t, propertyEncoders, propNames)
}

func (e *encoding) deriveDecoderForNamedObjectType(ref string, t tftypes.Object) (Decoder, error) {
	tok, refSpec, err := e.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	if refSpec.Enum != nil {
		return nil, fmt.Errorf("enums are not supported: %q", ref)
	}
	propNames := NewTypeLocalPropertyNames(e.propertyNames, tok)
	propertyDecoders, err := e.buildPropertyDecoders(propNames, specFinder(refSpec.Properties), t)
	if err != nil {
		return nil, fmt.Errorf("issue deriving an decoder for %q: %w", ref, err)
	}
	return newObjectDecoder(t, propertyDecoders, propNames)
}

func (e *encoding) deriveEncoder(typeSpec *pschema.TypeSpec, t tftypes.Type) (Encoder, error) {
	if (t.Is(tftypes.List{}) || t.Is(tftypes.Set{})) && typeSpec.Type != "array" {
		// For IsMaxItemOne lists or sets, Pulumi flattens List[T] or Set[T] to T.
		var elementType tftypes.Type
		if t.Is(tftypes.List{}) {
			elementType = t.(tftypes.List).ElementType
		} else {
			elementType = t.(tftypes.Set).ElementType
		}
		encoder, err := e.deriveEncoder(typeSpec, elementType)
		if err != nil {
			return nil, err
		}
		return &flattenedEncoder{
			collectionType: t,
			elementEncoder: encoder,
		}, nil
	}

	if typeSpec.Ref != "" {
		oT, ok := t.(tftypes.Object)
		if !ok {
			return nil, fmt.Errorf("expected Object type but got %s", t.String())
		}
		return e.deriveEncoderForNamedObjectType(typeSpec.Ref, oT)
	}
	switch typeSpec.Type {
	case "boolean":
		return newBoolEncoder(), nil
	case "integer":
		return newNumberEncoder(), nil // TODO integerEncoder
	case "number":
		return newNumberEncoder(), nil
	case "string":
		return newStringEncoder(), nil
	case "array":
		lt, ok := t.(tftypes.List)
		if !ok {
			return nil, fmt.Errorf("expected a List, got %s", t.String())
		}
		elementEncoder, err := e.deriveEncoder(typeSpec.Items, lt.ElementType)
		if err != nil {
			return nil, err
		}
		return newListEncoder(lt.ElementType, elementEncoder)
	case "object":
		// Ensure Map[string,X] type case
		if !(typeSpec.AdditionalProperties != nil && typeSpec.Ref == "") {
			return nil, fmt.Errorf("expected Ref or AdditionalProperties set")
		}
		mt, ok := t.(tftypes.Map)
		if !ok {
			return nil, fmt.Errorf("expected a Map, got %s", t.String())
		}
		elementEncoder, err := e.deriveEncoder(typeSpec.AdditionalProperties, mt.ElementType)
		if err != nil {
			return nil, err
		}
		return newMapEncoder(mt.ElementType, elementEncoder)
	default:
		return nil, fmt.Errorf("Cannot build an encoder for type %q", typeSpec.Type)
	}
}

func (e *encoding) deriveDecoder(typeSpec *pschema.TypeSpec, t tftypes.Type) (Decoder, error) {
	if (t.Is(tftypes.List{}) || t.Is(tftypes.Set{})) && typeSpec.Type != "array" {
		// In case of IsMaxItemOne lists or sets, Pulumi flattens List[T] or Set[T] to T.
		var elementType tftypes.Type
		if t.Is(tftypes.List{}) {
			elementType = t.(tftypes.List).ElementType
		} else {
			elementType = t.(tftypes.Set).ElementType
		}
		decoder, err := e.deriveDecoder(typeSpec, elementType)
		if err != nil {
			return nil, err
		}
		return &flattenedDecoder{
			elementDecoder: decoder,
		}, nil
	}

	if typeSpec.Ref != "" {
		oT, ok := t.(tftypes.Object)
		if !ok {
			return nil, fmt.Errorf("expected Object type but got %s", t.String())
		}
		return e.deriveDecoderForNamedObjectType(typeSpec.Ref, oT)
	}
	switch typeSpec.Type {
	case "boolean":
		return newBoolDecoder(), nil
	case "integer":
		return newNumberDecoder(), nil // TODO integerEncoder
	case "number":
		return newNumberDecoder(), nil
	case "string":
		return newStringDecoder(), nil
	case "array":
		lt, ok := t.(tftypes.List)
		if !ok {
			return nil, fmt.Errorf("expected a List, got %s", t.String())
		}
		elementDecoder, err := e.deriveDecoder(typeSpec.Items, lt.ElementType)
		if err != nil {
			return nil, err
		}
		return newListDecoder(elementDecoder)
	case "object":
		// Ensure Map[string,X] type case
		if !(typeSpec.AdditionalProperties != nil && typeSpec.Ref == "") {
			return nil, fmt.Errorf("expected Ref or AdditionalProperties set")
		}
		mt, ok := t.(tftypes.Map)
		if !ok {
			return nil, fmt.Errorf("expected a Map, got %s", t.String())
		}
		elementDecoder, err := e.deriveDecoder(typeSpec.AdditionalProperties, mt.ElementType)
		if err != nil {
			return nil, err
		}
		return newMapDecoder(elementDecoder)
	default:
		return nil, fmt.Errorf("Cannot build an ecoderfor type %q", typeSpec.Type)
	}
}

type renamedProperties map[tokens.Token]map[TerraformPropertyName]resource.PropertyKey

func (r renamedProperties) Renames(typ tokens.Token) map[TerraformPropertyName]resource.PropertyKey {
	return r[typ]
}

func (r renamedProperties) PropertyKey(typ tokens.Token, prop TerraformPropertyName) resource.PropertyKey {
	if m, ok := r[typ]; ok {
		if v, ok := m[prop]; ok {
			return v
		}
	}
	return resource.PropertyKey(prop)
}

type specFinderFn = func(pk resource.PropertyKey) *pschema.PropertySpec

func specFinderWithID(props map[string]pschema.PropertySpec) specFinderFn {
	return func(pk resource.PropertyKey) *pschema.PropertySpec {
		if prop, ok := props[string(pk)]; ok {
			return &prop
		}
		// Currently id is implied by the translation but absent from rspec.Properties.
		if string(pk) == "id" {
			return &pschema.PropertySpec{TypeSpec: pschema.TypeSpec{Type: "string"}}
		}
		return nil
	}
}

func specFinder(props map[string]pschema.PropertySpec) specFinderFn {
	return func(pk resource.PropertyKey) *pschema.PropertySpec {
		if prop, ok := props[string(pk)]; ok {
			return &prop
		}
		return nil
	}
}

func specFinderWithFallback(a, b specFinderFn) specFinderFn {
	return func(pk resource.PropertyKey) *pschema.PropertySpec {
		v := a(pk)
		if v != nil {
			return v
		}
		return b(pk)
	}
}
