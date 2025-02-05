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

// Converts between Pulumi and Terraform value representations. Conversions are type-driven and require to know
// Terraform types since Terraform values must be tagged with their corresponding types. Pulumi type metadata is also
// required to make finer grained distinctions such as int vs float, and to correctly handle object properties that have
// Pulumi names that differ from their Terraform names.
package convert

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// An alias to assist marking Terraform-level property names (see for example AttributeTypes in tftypes.Object). Pulumi
// may rename properties and it is important to keep track of which name is being used during conversion.
type terraformPropertyName = string

type Encoding interface {
	NewConfigEncoder(tftypes.Object) (Encoder, error)
	NewResourceDecoder(resource string, resourceType tftypes.Object) (Decoder, error)
	NewResourceEncoder(resource string, resourceType tftypes.Object) (Encoder, error)
	NewDataSourceDecoder(dataSource string, dataSourceType tftypes.Object) (Decoder, error)
	NewDataSourceEncoder(dataSource string, dataSourceType tftypes.Object) (Encoder, error)
}

// Like PropertyNames but specialized to either a type by token or config property.
type localPropertyNames interface {
	PropertyKey(property terraformPropertyName, t tftypes.Type) resource.PropertyKey
}

type Encoder interface {
	fromPropertyValue(resource.PropertyValue) (tftypes.Value, error)
}

// Schema information that is needed to construct Encoder or Decoder instances.
type ObjectSchema struct {
	SchemaMap   shim.SchemaMap                  // required
	SchemaInfos map[string]*tfbridge.SchemaInfo // optional
	Object      *tftypes.Object                 // optional, if not given will be inferred from SchemaMap
}

func (os ObjectSchema) objectType() tftypes.Object {
	if os.Object != nil {
		return *os.Object
	}
	return InferObjectType(os.SchemaMap, nil)
}

func NewObjectEncoder(os ObjectSchema) (Encoder, error) {
	mctx := newSchemaMapContext(os.SchemaMap, os.SchemaInfos)
	objectType := os.objectType()
	propertyEncoders, err := buildPropertyEncoders(mctx, objectType)
	if err != nil {
		return nil, err
	}
	enc, err := newObjectEncoder(objectType, propertyEncoders, mctx)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

type Decoder interface {
	// toPropertyValue applies the conversion of the Decoder.
	//
	// toPropertyValue should not be called directly. Instead, it should be called
	// through [decode].
	//
	// You *should* do this:
	//
	//	var myDecoder Decoder = ...
	//	var myValue tftypes.Value = ...
	//	return decode(myDecoder, value)
	//
	// You *should not* do this:
	//
	//	var myDecoder Decoder = ...
	//	var myValue tftypes.Value = ...
	//	return myDecoder.toPropertyValue(value)
	//
	// This is because toPropertyValue is not required to handle [tftypes.Value]
	// modifiers, like secrets and unknown values.
	toPropertyValue(tftypes.Value) (resource.PropertyValue, error)
}

func NewObjectDecoder(os ObjectSchema) (Decoder, error) {
	objectType := os.objectType()
	mctx := newSchemaMapContext(os.SchemaMap, os.SchemaInfos)
	propertyDecoders, err := buildPropertyDecoders(mctx, objectType)
	if err != nil {
		return nil, err
	}
	dec, err := newObjectDecoder(objectType, propertyDecoders, mctx)
	if err != nil {
		return nil, err
	}
	return dec, nil
}

func EncodePropertyMap(enc Encoder, pmap resource.PropertyMap) (tftypes.Value, error) {
	return enc.fromPropertyValue(propertyvalue.RemoveSecrets(resource.NewObjectProperty(pmap)))
}

func decode(dec Decoder, v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	return dec.toPropertyValue(v)
}

func DecodePropertyMap(ctx context.Context, dec Decoder, v tftypes.Value) (resource.PropertyMap, error) {
	pv, err := decode(dec, v)
	if err != nil {
		return nil, err
	}
	if !pv.IsObject() {
		details := fmt.Sprintf(`DecodePropertyMap expected the decoder to return an Object
PropertyValue returned by the decoder: %v
tftypes.Value passed to the decoder:   %v`, pv.String(), v.String())
		tfbridge.GetLogger(ctx).Debug(details)
		return nil, fmt.Errorf(`Internal error: DecodePropertyMap expected the decoder to return an Object.
Please report to https://github.com/pulumi/pulumi-terraform-bridge/issues`)
	}
	return pv.ObjectValue(), nil
}

func EncodePropertyMapToDynamic(enc Encoder, objectType tftypes.Object,
	pmap resource.PropertyMap,
) (*tfprotov6.DynamicValue, error) {
	v, err := EncodePropertyMap(enc, pmap)
	if err != nil {
		return nil, err
	}
	dv, err := tfprotov6.NewDynamicValue(objectType, v)
	return &dv, err
}

func DecodePropertyMapFromDynamic(
	ctx context.Context,
	dec Decoder,
	objectType tftypes.Object,
	dv *tfprotov6.DynamicValue,
) (resource.PropertyMap, error) {
	v, err := dv.Unmarshal(objectType)
	if err != nil {
		return nil, fmt.Errorf("DynamicValue.Unmarshal failed: %w", err)
	}
	return DecodePropertyMap(ctx, dec, v)
}
