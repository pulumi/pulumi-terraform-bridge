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

func (e *encoding) NewResourceEncoder(resourceToken tokens.Type, objectType tftypes.Object) (Encoder, error) {
	rspec := e.spec.Resource(resourceToken)
	if rspec == nil {
		return nil, fmt.Errorf("dangling resource token %q", string(resourceToken))
	}
	spec := specFinderWithID(rspec.Properties)
	propertyEncoders, err := e.buildPropertyEncoders(tokens.Token(resourceToken), spec, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for resource %q: %w", string(resourceToken), err)
	}
	enc, err := newObjectEncoder(tokens.Token(resourceToken), objectType, propertyEncoders, e.propertyNames)
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
	propertyDecoders, err := e.buildPropertyDecoders(tokens.Token(resourceToken), spec, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an decoder for resource %q: %w", string(resourceToken), err)
	}
	propertyDecoders["id"] = newStringDecoder()
	dec, err := newObjectDecoder(tokens.Token(resourceToken), objectType, propertyDecoders, e.propertyNames)
	if err != nil {
		return nil, fmt.Errorf("cannot derive a decoder for resource %q: %w", string(resourceToken), err)
	}
	return dec, nil
}

func (e *encoding) buildPropertyEncoders(
	token tokens.Token,
	propSpecs func(resource.PropertyKey) *pschema.PropertySpec,
	objectType tftypes.Object,
) (map[TerraformPropertyName]Encoder, error) {
	propertyEncoders := map[TerraformPropertyName]Encoder{}
	for tfName, t := range objectType.AttributeTypes {
		key := e.propertyNames.PropertyKey(token, tfName, t)
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
	token tokens.Token,
	propSpecs func(resource.PropertyKey) *pschema.PropertySpec,
	objectType tftypes.Object,
) (map[TerraformPropertyName]Decoder, error) {
	propertyEncoders := map[TerraformPropertyName]Decoder{}
	for tfName, t := range objectType.AttributeTypes {
		key := e.propertyNames.PropertyKey(token, tfName, t)
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
	propertyEncoders, err := e.buildPropertyEncoders(tok, specFinder(refSpec.Properties), t)
	if err != nil {
		return nil, fmt.Errorf("issue deriving an encoder for %q: %w", ref, err)
	}
	return newObjectEncoder(tok, t, propertyEncoders, e.propertyNames)
}

func (e *encoding) deriveDecoderForNamedObjectType(ref string, t tftypes.Object) (Decoder, error) {
	tok, refSpec, err := e.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	if refSpec.Enum != nil {
		return nil, fmt.Errorf("enums are not supported: %q", ref)
	}
	propertyDecoders, err := e.buildPropertyDecoders(tok, specFinder(refSpec.Properties), t)
	if err != nil {
		return nil, fmt.Errorf("issue deriving an decoder for %q: %w", ref, err)
	}
	return newObjectDecoder(tok, t, propertyDecoders, e.propertyNames)
}

func (e *encoding) deriveEncoder(typeSpec *pschema.TypeSpec, t tftypes.Type) (Encoder, error) {
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

func specFinderWithID(props map[string]pschema.PropertySpec) func(pk resource.PropertyKey) *pschema.PropertySpec {
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

func specFinder(props map[string]pschema.PropertySpec) func(pk resource.PropertyKey) *pschema.PropertySpec {
	return func(pk resource.PropertyKey) *pschema.PropertySpec {
		if prop, ok := props[string(pk)]; ok {
			return &prop
		}
		return nil
	}
}
