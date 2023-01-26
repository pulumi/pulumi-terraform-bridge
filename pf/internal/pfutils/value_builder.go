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

package pfutils

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type valueBuilder func(tftypes.Type) tftypes.Value

type fieldBuilder struct {
	fieldName    string
	valueBuilder valueBuilder
}

func object(fields map[string]valueBuilder) valueBuilder {
	if fields == nil {
		fields = map[string]valueBuilder{}
	}
	return func(t tftypes.Type) tftypes.Value {
		oT := t.(tftypes.Object)
		values := map[string]tftypes.Value{}
		for k, f := range fields {
			v := f(oT.AttributeTypes[k])
			values[k] = v
		}
		for k, v := range oT.AttributeTypes {
			_, opt := oT.OptionalAttributes[k]
			_, set := values[k]
			if !opt && !set {
				values[k] = tftypes.NewValue(v, nil)
			}
		}
		return tftypes.NewValue(t, values)
	}
}

func obj(fields ...fieldBuilder) valueBuilder {
	return func(t tftypes.Type) tftypes.Value {
		oT := t.(tftypes.Object)
		values := map[string]tftypes.Value{}
		for _, f := range fields {
			v := f.valueBuilder(oT.AttributeTypes[f.fieldName])
			values[f.fieldName] = v
		}
		return tftypes.NewValue(t, values)
	}
}

func field(name string, valueBuilder valueBuilder) fieldBuilder {
	return fieldBuilder{name, valueBuilder}
}

func prim(val interface{}) valueBuilder {
	return func(t tftypes.Type) tftypes.Value {
		return tftypes.NewValue(t, val)
	}
}

func unk() valueBuilder {
	return func(t tftypes.Type) tftypes.Value {
		return tftypes.NewValue(t, tftypes.UnknownValue)
	}
}

func mapv(elements map[string]valueBuilder) valueBuilder {
	if elements == nil {
		elements = map[string]valueBuilder{}
	}
	return func(t tftypes.Type) tftypes.Value {
		eT := t.(tftypes.Map).ElementType
		pieces := map[string]tftypes.Value{}
		for k, v := range elements {
			pieces[k] = v(eT)
		}
		return tftypes.NewValue(t, pieces)
	}
}

func list(elems ...valueBuilder) valueBuilder {
	return func(t tftypes.Type) tftypes.Value {
		eT := t.(tftypes.List).ElementType
		var values []tftypes.Value
		for _, b := range elems {
			values = append(values, b(eT))
		}
		return tftypes.NewValue(t, values)
	}
}

func set(elems ...valueBuilder) valueBuilder {
	return func(t tftypes.Type) tftypes.Value {
		eT := t.(tftypes.Set).ElementType
		var values []tftypes.Value
		for _, b := range elems {
			values = append(values, b(eT))
		}
		return tftypes.NewValue(t, values)
	}
}
