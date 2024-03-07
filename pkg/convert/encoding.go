// Copyright 2016-2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package convert

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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
	schema := e.SchemaOnlyProvider.Schema()
	schemaInfos := e.ProviderInfo.Config
	enc, err := NewObjectEncoder(schema, schemaInfos, configType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for provider config: %w", err)
	}
	return enc, nil
}

func (e *encoding) NewResourceEncoder(resource string, objectType tftypes.Object) (Encoder, error) {
	mctx := newResourceSchemaMapContext(resource, e.SchemaOnlyProvider, e.ProviderInfo)
	enc, err := NewObjectEncoder(mctx.schemaMap, mctx.schemaInfos, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for resource %q: %w",
			resource, err)
	}
	return enc, nil
}

func (e *encoding) NewResourceDecoder(resource string, objectType tftypes.Object) (Decoder, error) {
	mctx := newResourceSchemaMapContext(resource, e.SchemaOnlyProvider, e.ProviderInfo)
	dec, err := NewObjectDecoder(mctx.schemaMap, mctx.schemaInfos, objectType)
	// Tests pass without this line. Was this intentional? Perhaps to support resources with
	// non-string IDs? propertyDecoders["id"] = newStringDecoder().
	if err != nil {
		return nil, fmt.Errorf("cannot derive a decoder for resource %q: %w", resource, err)
	}
	return dec, nil
}

func (e *encoding) NewDataSourceEncoder(
	dataSource string, objectType tftypes.Object,
) (Encoder, error) {
	mctx := newDataSourceSchemaMapContext(dataSource, e.SchemaOnlyProvider, e.ProviderInfo)
	enc, err := NewObjectEncoder(mctx.schemaMap, mctx.schemaInfos, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an encoder for data source %q: %w",
			dataSource, err)
	}
	return enc, nil
}

func (e *encoding) NewDataSourceDecoder(
	dataSource string, objectType tftypes.Object,
) (Decoder, error) {
	mctx := newDataSourceSchemaMapContext(dataSource, e.SchemaOnlyProvider, e.ProviderInfo)
	dec, err := NewObjectDecoder(mctx.schemaMap, mctx.schemaInfos, objectType)
	if err != nil {
		return nil, fmt.Errorf("cannot derive an decoder for data source %q: %w",
			dataSource, err)
	}
	return dec, nil
}

func buildPropertyEncoders(
	mctx *schemaMapContext, objectType tftypes.Object,
) (map[terraformPropertyName]Encoder, error) {
	propertyEncoders := map[terraformPropertyName]Encoder{}
	for tfName, t := range objectType.AttributeTypes {
		pctx, err := mctx.GetAttr(tfName)
		if err != nil {
			return nil, err
		}
		enc, err := newPropertyEncoder(pctx, tfName, t)
		if err != nil {
			return nil, err
		}
		propertyEncoders[tfName] = enc
	}
	return propertyEncoders, nil
}

func buildPropertyDecoders(
	mctx *schemaMapContext, objectType tftypes.Object,
) (map[terraformPropertyName]Decoder, error) {
	propertyEncoders := map[terraformPropertyName]Decoder{}
	for tfName, t := range objectType.AttributeTypes {
		pctx, err := mctx.GetAttr(tfName)
		if err != nil {
			return nil, err
		}
		dec, err := newPropertyDecoder(pctx, tfName, t)
		if err != nil {
			return nil, err
		}
		propertyEncoders[tfName] = dec
	}
	return propertyEncoders, nil
}

func newPropertyEncoder(
	pctx *schemaPropContext, name terraformPropertyName, t tftypes.Type,
) (Encoder, error) {
	enc, err := deriveEncoder(pctx, t)
	if err != nil {
		return nil, fmt.Errorf("Cannot derive an encoder for property %q: %w", name, err)
	}
	return enc, nil
}

func newPropertyDecoder(pctx *schemaPropContext, name terraformPropertyName,
	t tftypes.Type) (Decoder, error) {
	dec, err := deriveDecoder(pctx, t)
	if err != nil {
		return nil, fmt.Errorf("Cannot derive a decoder for property %q: %w", name, err)
	}
	if pctx.Secret() {
		return newSecretDecoder(dec)
	}
	return dec, nil
}

func deriveEncoder(pctx *schemaPropContext, t tftypes.Type) (Encoder, error) {
	if elementType, mio := pctx.IsMaxItemsOne(t); mio {
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		encoder, err := deriveEncoder(elctx, elementType)
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
	}

	switch tt := t.(type) {
	case tftypes.Object:
		mctx, err := pctx.Object()
		if err != nil {
			return nil, fmt.Errorf("issue deriving an object encoder: %w", err)
		}
		propertyEncoders, err := buildPropertyEncoders(mctx, tt)
		if err != nil {
			return nil, fmt.Errorf("issue deriving an object encoder: %w", err)
		}
		return newObjectEncoder(tt, propertyEncoders, mctx)
	case tftypes.List:
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		elementEncoder, err := deriveEncoder(elctx, tt.ElementType)
		if err != nil {
			return nil, err
		}
		return newListEncoder(tt.ElementType, elementEncoder)
	case tftypes.Map:
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		elementEncoder, err := deriveEncoder(elctx, tt.ElementType)
		if err != nil {
			return nil, err
		}
		return newMapEncoder(tt.ElementType, elementEncoder)
	case tftypes.Set:
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		elementEncoder, err := deriveEncoder(elctx, tt.ElementType)
		if err != nil {
			return nil, err
		}
		return newSetEncoder(tt.ElementType, elementEncoder)
	case tftypes.Tuple:
		return deriveTupleEncoder(pctx, tt)
	default:
		return nil, fmt.Errorf("Cannot build an encoder for type %v", t)
	}
}

func deriveDecoder(pctx *schemaPropContext, t tftypes.Type) (Decoder, error) {
	if elementType, mio := pctx.IsMaxItemsOne(t); mio {
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		decoder, err := deriveDecoder(elctx, elementType)
		if err != nil {
			return nil, err
		}
		return &flattenedDecoder{
			elementDecoder: decoder,
		}, nil
	}

	switch {
	case t.Is(tftypes.String):
		return newStringDecoder(), nil
	case t.Is(tftypes.Number):
		return newNumberDecoder(), nil
	case t.Is(tftypes.Bool):
		return newBoolDecoder(), nil
	}

	switch tt := t.(type) {
	case tftypes.Object:
		mctx, err := pctx.Object()
		if err != nil {
			return nil, fmt.Errorf("issue deriving an object encoder: %w", err)
		}
		propertyDecoders, err := buildPropertyDecoders(mctx, tt)
		if err != nil {
			return nil, fmt.Errorf("issue deriving an object encoder: %w", err)
		}
		return newObjectDecoder(tt, propertyDecoders, mctx)
	case tftypes.List:
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		elementDecoder, err := deriveDecoder(elctx, tt.ElementType)
		if err != nil {
			return nil, err
		}
		return newListDecoder(elementDecoder)
	case tftypes.Map:
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		elementDecoder, err := deriveDecoder(elctx, tt.ElementType)
		if err != nil {
			return nil, err
		}
		return newMapDecoder(elementDecoder)
	case tftypes.Set:
		elctx, err := pctx.Element()
		if err != nil {
			return nil, err
		}
		elementDecoder, err := deriveDecoder(elctx, tt.ElementType)
		if err != nil {
			return nil, err
		}
		return newSetDecoder(elementDecoder)
	case tftypes.Tuple:
		return deriveTupleDecoder(pctx, tt)
	default:
		return nil, fmt.Errorf("Cannot build a decoder type %v", t)
	}
}

// A generic base function for deriving tuple encoders and decoders.
//
// It handles reference validation and property discovery.
func deriveTupleBase[T any](
	pctx *schemaPropContext,
	f func(*schemaPropContext, tftypes.Type) (T, error),
	t tftypes.Tuple,
) ([]T, error) {
	elements := make([]T, len(t.ElementTypes))
	for i := range t.ElementTypes {
		var err error
		elctx, err := pctx.TupleElement(i)
		if err != nil {
			return nil, err
		}
		elements[i], err = f(elctx, t.ElementTypes[i])
		if err != nil {
			return nil, err
		}
	}
	return elements, nil
}

func deriveTupleEncoder(
	pctx *schemaPropContext, t tftypes.Tuple,
) (*tupleEncoder, error) {
	encoders, err := deriveTupleBase(pctx, deriveEncoder, t)
	if err != nil {
		return nil, fmt.Errorf("could not build tuple encoder: %w", err)
	}
	return &tupleEncoder{t.ElementTypes, encoders}, nil
}

func deriveTupleDecoder(
	pctx *schemaPropContext, t tftypes.Tuple,
) (*tupleDecoder, error) {
	decoders, err := deriveTupleBase(pctx, deriveDecoder, t)
	if err != nil {
		return nil, fmt.Errorf("could not build tuple decoder: %w", err)
	}
	return &tupleDecoder{decoders}, nil
}
