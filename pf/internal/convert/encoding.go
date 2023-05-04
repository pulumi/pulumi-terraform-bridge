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
	//"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type encoding struct {
	SchemaOnlyProvider shim.Provider
	ProviderInfo       *tfbridge.ProviderInfo // only SchemaInfo for fields is required
}

var _ Encoding = (*encoding)(nil)

func NewEncoding(schemaOnlyProvider shim.Provider, providerInfo *tfbridge.ProviderInfo) Encoding {
	return &encoding{
		SchemaOnlyProvider: schemaOnlyProvider,
		ProviderInfo:       providerInfo,
	}
}

func (e *encoding) NewConfigEncoder(configType tftypes.Object) (Encoder, error) {
	// spec := specFinder(e.spec.Config().Variables)
	// propNames := NewConfigPropertyNames(e.propertyNames)
	propertyEncoders, err := e.buildPropertyEncoders(nil /* TODO */, configType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for provider config: %w", err)
	}
	enc, err := newObjectEncoder(configType, propertyEncoders, nil /* TODO */)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for provider config: %w", err)
	}
	return enc, nil
}

func (e *encoding) NewResourceEncoder(resourceToken tokens.Type, /* TODO tf ID */
	objectType tftypes.Object) (Encoder, error) {

	// rspec := e.spec.Resource(resourceToken)
	// if rspec == nil {
	// 	return nil, fmt.Errorf("dangling resource token %q", string(resourceToken))
	// }
	// spec := specFinderWithID(rspec.Properties)
	// propNames := NewTypeLocalPropertyNames(e.propertyNames, tokens.Token(resourceToken))
	propertyEncoders, err := e.buildPropertyEncoders(nil /* TODO */, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for resource %q: %w", string(resourceToken), err)
	}
	enc, err := newObjectEncoder(objectType, propertyEncoders, nil /* TODO */)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for resource %q: %w", string(resourceToken), err)
	}
	return enc, nil
}

func (e *encoding) NewResourceDecoder(resourceToken tokens.Type, objectType tftypes.Object) (Decoder, error) {
	// rspec := e.spec.Resource(resourceToken)
	// if rspec == nil {
	// 	return nil, fmt.Errorf("dangling resource token %q", string(resourceToken))
	// }
	// spec := specFinderWithID(rspec.Properties)
	// propNames := NewTypeLocalPropertyNames(nil /* TODO */, tokens.Token(resourceToken))
	propertyDecoders, err := e.buildPropertyDecoders(nil /* TODO */, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an decoder for resource %q: %w", string(resourceToken), err)
	}
	propertyDecoders["id"] = newStringDecoder()
	dec, err := newObjectDecoder(objectType, propertyDecoders, nil /* TODO */)
	if err != nil {
		return nil, fmt.Errorf("cannot derive a decoder for resource %q: %w", string(resourceToken), err)
	}
	return dec, nil
}

func (e *encoding) NewDataSourceEncoder(functionToken tokens.ModuleMember, objectType tftypes.Object) (Encoder, error) {
	// fspec := e.spec.Function(functionToken)
	// if fspec == nil {
	// 	return nil, fmt.Errorf("dangling function token %q", string(functionToken))
	// }
	token := tokens.Token(functionToken)
	// spec := specFinderWithFallback(specFinder(fspec.Inputs.Properties), specFinder(functionOutputs(fspec).Properties))
	// propNames := NewTypeLocalPropertyNames(e.propertyNames, token)
	propertyEncoders, err := e.buildPropertyEncoders(nil /* TODO */, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for function %q: %w", string(token), err)
	}
	enc, err := newObjectEncoder(objectType, propertyEncoders, nil /* TODO */)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for function %q: %w", string(token), err)
	}
	return enc, nil
}

func (e *encoding) NewDataSourceDecoder(functionToken tokens.ModuleMember, objectType tftypes.Object) (Decoder, error) {
	token := tokens.Token(functionToken)
	// fspec := e.spec.Function(functionToken)
	// if fspec == nil {
	// 	return nil, fmt.Errorf("dangling function token %q", string(token))
	// }
	//spec := specFinderWithFallback(specFinder(functionOutputs(fspec).Properties), specFinder(fspec.Inputs.Properties))
	// propNames := NewTypeLocalPropertyNames(e.propertyNames, token)
	propertyDecoders, err := e.buildPropertyDecoders(nil /* TODO */, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an decoder for function %q: %w", string(token), err)
	}
	dec, err := newObjectDecoder(objectType, propertyDecoders, nil /* TODO */)
	if err != nil {
		return nil, fmt.Errorf("cannot derive a decoder for function %q: %w", string(token), err)
	}
	return dec, nil
}

func (e *encoding) buildPropertyEncoders(
	mctx *schemaMapContext,
	objectType tftypes.Object,
) (map[TerraformPropertyName]Encoder, error) {
	propertyEncoders := map[TerraformPropertyName]Encoder{}
	/* TODO */
	// for tfName, t := range objectType.AttributeTypes {
	// 	pctx := mctx.GetAttr(tfName)
	// 	key := mctx.ToPropertyKey(tfName)
	// 	enc, err := e.newPropertyEncoder(tfName, *puSpec, t)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	propertyEncoders[tfName] = enc
	// }
	return propertyEncoders, nil
}

func (e *encoding) buildPropertyDecoders(
	mctx *schemaMapContext,
	objectType tftypes.Object,
) (map[TerraformPropertyName]Decoder, error) {
	/* TODO */
	// propertyEncoders := map[TerraformPropertyName]Decoder{}
	// for tfName, t := range objectType.AttributeTypes {
	// 	key := propertyNames.PropertyKey(tfName, t)
	// 	puSpec := propSpecs(key)
	// 	if puSpec == nil {
	// 		return nil, fmt.Errorf("missing property %q", string(key))
	// 	}
	// 	dec, err := e.newPropertyDecoder(tfName, *puSpec, t)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	propertyEncoders[tfName] = dec
	// }
	// return propertyEncoders, nil
	panic("TOOD")
}

func (e *encoding) newPropertyEncoder(pctx *schemaPropContext, name TerraformPropertyName,
	t tftypes.Type) (Encoder, error) {
	enc, err := e.deriveEncoder(pctx, t)
	if err != nil {
		return nil, fmt.Errorf("Cannot derive an encoder for property %q: %w", name, err)
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

func (e *encoding) deriveEncoder(pctx *schemaPropContext, t tftypes.Type) (Encoder, error) {
	if elementType, mio := pctx.IsMaxItemsOne(t); mio {
		encoder, err := e.deriveEncoder(pctx.Element(), elementType)
		if err != nil {
			return nil, err
		}
		return &flattenedEncoder{
			collectionType: t,
			elementEncoder: encoder,
		}, nil
	}

	switch {
	case t.Is(tftypes.String):
		return newStringEncoder(), nil
	case t.Is(tftypes.Number):
		return newNumberEncoder(), nil
	case t.Is(tftypes.Bool):
		return newBoolEncoder(), nil
	default:
		switch tt := t.(type) {
		case tftypes.Object:
			propertyEncoders, err := e.buildPropertyEncoders(pctx.Object(), tt)
			if err != nil {
				return nil, fmt.Errorf("issue deriving an object encoder: %w", err)
			}
			return newObjectEncoder(tt, propertyEncoders, nil)
		case tftypes.List:
			elementEncoder, err := e.deriveEncoder(pctx.Element(), tt.ElementType)
			if err != nil {
				return nil, err
			}
			return newListEncoder(tt.ElementType, elementEncoder)
		case tftypes.Map:
			elementEncoder, err := e.deriveEncoder(pctx.Element(), tt.ElementType)
			if err != nil {
				return nil, err
			}
			return newMapEncoder(tt.ElementType, elementEncoder)
		case tftypes.Set:
			elementEncoder, err := e.deriveEncoder(pctx.Element(), tt.ElementType)
			if err != nil {
				return nil, err
			}
			return newSetEncoder(tt.ElementType, elementEncoder)
		case tftypes.Tuple:
			// 	tok, referredType, err := e.resolveRef(typeSpec.Ref)
			// 	if err != nil {
			// 		return nil, fmt.Errorf("expected a Tuple type: %w", err)
			// 	}
			// 	return e.deriveTupleEncoder(tokens.Type(tok), referredType, t)
			panic("tuple")
		default:
			return nil, fmt.Errorf("Cannot build an encoder for type %s", t)
		}
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

	// switch t := t.(type) {
	// case tftypes.Object:
	// 	ref, referredType, err := e.resolveRef(typeSpec.Ref)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("expected an Object type: %w", err)
	// 	}
	// 	return e.deriveDecoderForNamedObjectType(ref, referredType, t)
	// case tftypes.Tuple:
	// 	ref, referredType, err := e.resolveRef(typeSpec.Ref)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("expected a Tuple type: %w", err)
	// 	}
	// 	return e.deriveTupleDecoder(tokens.Type(ref), referredType, t)
	// }

	switch typeSpec.Type {
	case "boolean":
		return newBoolDecoder(), nil
	case "integer":
		return newNumberDecoder(), nil
	case "number":
		return newNumberDecoder(), nil
	case "string":
		return newStringDecoder(), nil
	case "array":
		switch t := t.(type) {
		case tftypes.List:
			elementDecoder, err := e.deriveDecoder(typeSpec.Items, t.ElementType)
			if err != nil {
				return nil, err
			}
			return newListDecoder(elementDecoder)
		case tftypes.Set:
			elementDecoder, err := e.deriveDecoder(typeSpec.Items, t.ElementType)
			if err != nil {
				return nil, err
			}
			return newSetDecoder(elementDecoder)
		default:
			return nil, fmt.Errorf("expected a List or Set, got %s", t.String())
		}
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
		return nil, fmt.Errorf("Cannot build a decoder type %q", typeSpec.Type)
	}
}

// A generic base function for deriving tuple encoders and decoders.
//
// It handles reference validation and property discovery.
func deriveTupleBase[T any](
	f func(*pschema.TypeSpec, tftypes.Type) (T, error),
	tok tokens.Type,
	typ *pschema.ComplexTypeSpec,
	t tftypes.Tuple,
) ([]T, error) {
	elements := make([]T, len(t.ElementTypes))
	for i := range t.ElementTypes {
		propName := tuplePropertyName(i)
		prop, ok := typ.Properties[propName]
		if !ok {
			return nil, fmt.Errorf("could not find expected property '%s' on type '%s'",
				propName, tok)
		}
		var err error
		elements[i], err = f(&prop.TypeSpec, t.ElementTypes[i])
		if err != nil {
			return nil, err
		}
	}
	return elements, nil
}

func (e *encoding) deriveTupleEncoder(tok tokens.Type, typeSpec *pschema.ComplexTypeSpec,
	t tftypes.Tuple) (*tupleEncoder, error) {
	panic("TODO")
	// encoders, err := deriveTupleBase(e.deriveEncoder, tok, typeSpec, t)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not build tuple encoder: %w", err)
	// }
	// return &tupleEncoder{t.ElementTypes, encoders}, nil
}

func (e *encoding) deriveTupleDecoder(tok tokens.Type, typeSpec *pschema.ComplexTypeSpec,
	t tftypes.Tuple) (*tupleDecoder, error) {
	decoders, err := deriveTupleBase(e.deriveDecoder, tok, typeSpec, t)
	if err != nil {
		return nil, fmt.Errorf("could not build tuple decoder: %w", err)
	}
	return &tupleDecoder{decoders}, nil
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
