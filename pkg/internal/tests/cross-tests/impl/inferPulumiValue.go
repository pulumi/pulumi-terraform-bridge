// Copyright 2016-2024, Pulumi Corporation.
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

// Package crosstestsimpl (cross-tests implementation) contains code meant to be shared
// across cross-test implementations (SDKv2, PF) but not used by people writing tests
// themselves.
package crosstestsimpl

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

// InferPulumiValue generates a Pulumi value that is semantically equivalent to v.
//
// InferPulumiValue takes into account schema information.
func InferPulumiValue(t T, schema shim.SchemaMap, infos map[string]*info.Schema, v cty.Value) resource.PropertyMap {
	if v.IsNull() {
		return nil
	}
	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   schema,
		SchemaInfos: infos,
	})
	require.NoError(t, err)

	ctx := logging.InitLogging(context.Background(), logging.LogOptions{})
	// There is not yet a way to opt out of marking schema secrets, so the resulting map might have secrets marked.
	pm, err := convert.DecodePropertyMap(ctx, decoder, ctyToTftypes(v))
	require.NoError(t, err)
	return pm
}

func ctyToTftypes(v cty.Value) tftypes.Value {
	typ := v.Type()
	if !v.IsKnown() {
		return tftypes.NewValue(ctyTypeToTfType(typ), tftypes.UnknownValue)
	}
	if v.IsNull() {
		return tftypes.NewValue(ctyTypeToTfType(typ), nil)
	}
	switch {
	case typ.Equals(cty.String):
		return tftypes.NewValue(ctyTypeToTfType(typ), v.AsString())
	case typ.Equals(cty.Bool):
		return tftypes.NewValue(ctyTypeToTfType(typ), v.True())
	case typ.Equals(cty.Number):
		return tftypes.NewValue(ctyTypeToTfType(typ), v.AsBigFloat())

	case typ.IsListType():
		src := v.AsValueSlice()
		dst := make([]tftypes.Value, len(src))
		for i, v := range src {
			dst[i] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	case typ.IsSetType():
		src := v.AsValueSet().Values()
		dst := make([]tftypes.Value, len(src))
		for i, v := range src {
			dst[i] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	case typ.IsMapType():
		src := v.AsValueMap()
		dst := make(map[string]tftypes.Value, len(src))
		for k, v := range src {
			dst[k] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	case typ.IsObjectType():
		src := v.AsValueMap()
		dst := make(map[string]tftypes.Value, len(src))
		for k, v := range src {
			dst[k] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	default:
		panic(fmt.Sprintf("unknown type %s", typ.GoString()))
	}
}

func ctyTypeToTfType(typ cty.Type) tftypes.Type {
	switch {
	case typ.Equals(cty.String):
		return tftypes.String
	case typ.Equals(cty.Bool):
		return tftypes.Bool
	case typ.Equals(cty.Number):
		return tftypes.Number
	case typ == cty.DynamicPseudoType:
		return tftypes.DynamicPseudoType

	case typ.IsListType():
		return tftypes.List{ElementType: ctyTypeToTfType(typ.ElementType())}
	case typ.IsSetType():
		return tftypes.Set{ElementType: ctyTypeToTfType(typ.ElementType())}
	case typ.IsMapType():
		return tftypes.Map{ElementType: ctyTypeToTfType(typ.ElementType())}
	case typ.IsObjectType():
		src := typ.AttributeTypes()
		dst := make(map[string]tftypes.Type, len(src))
		for k, v := range src {
			dst[k] = ctyTypeToTfType(v)
		}
		return tftypes.Object{AttributeTypes: dst}
	default:
		panic(fmt.Sprintf("unknown type %s", typ.GoString()))
	}
}
