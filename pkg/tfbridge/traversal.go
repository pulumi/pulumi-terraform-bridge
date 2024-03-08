// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridge

import (
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	tfshimutil "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func NewResourceTraversal(ps shim.Resource, info *ResourceInfo) FullResource {
	var fields map[string]*SchemaInfo
	if info != nil {
		fields = info.Fields
	}
	obj := FullObject{
		schema: (&schema.Schema{Elem: ps, Type: shim.TypeMap}).Shim(),
		info:   &SchemaInfo{Fields: fields},
	}

	return FullResource{
		FullObject:   obj,
		ResourceShim: ps,
		ResourceInfo: info,
	}
}

func NewObjectTraversal(tfs shim.SchemaMap, info map[string]*SchemaInfo) FullObject {
	r := (&schema.Resource{Schema: tfs}).Shim()
	return FullObject{
		schema: (&schema.Schema{Elem: r, Type: shim.TypeMap}).Shim(),
		info:   &SchemaInfo{Fields: info},
	}

}

type FullResource struct {
	FullObject
	ResourceShim shim.Resource
	ResourceInfo *ResourceInfo
}

type FullSchema struct {
	schema shim.Schema
	info   *SchemaInfo
}

func (f FullSchema) Info() *SchemaInfo     { return f.info }
func (f FullSchema) TFSchema() shim.Schema { return f.schema }
func (f FullSchema) isElemType()           {}

func (f FullSchema) IsMaxItemsOne() bool { return IsMaxItemsOne(f.schema, f.info) }

type FullObject struct {
	// A schema that represents an object type.
	//
	// It *must* be the case that `_, ok := tfshimutil.CastToTypeObject(schema); ok`.
	schema shim.Schema

	// The SchemaInfo associated with the object
	info *SchemaInfo
}

func (f FullObject) Info() *SchemaInfo { return f.info }
func (f FullObject) asObj() shim.SchemaMap {
	if f.schema == nil {
		return schema.SchemaMap{}
	}
	obj, ok := tfshimutil.CastToTypeObject(f.schema)
	contract.Assertf(ok, "f.schema must be a valid object")
	return obj
}

func (f FullObject) Fields() (shim.SchemaMap, map[string]*SchemaInfo) {
	return f.asObj(), f.fieldsInfo()
}

func (f FullObject) isElemType() {}

func (f FullObject) fieldsInfo() map[string]*SchemaInfo {
	if f.info == nil {
		return nil
	}
	return f.info.Fields
}

type FullElem interface {
	TFSchema() shim.Schema
	Info() *SchemaInfo
	isElemType()
}

// MissingElem represents a missing element, either from calling FullObject.Field(key)
// where key does not exist or calling FullSchema.Elem() where FullSchema.Elem is nil on
// the underlying shim.Schema.
type MissingElem struct{}

func (MissingElem) TFSchema() shim.Schema { return nil }
func (MissingElem) Info() *SchemaInfo     { return nil }
func (MissingElem) isElemType()           {}

func (f FullObject) TFSchema() shim.Schema { return f.schema }

func (f FullObject) Field(tfName string) FullElem {
	var info *SchemaInfo
	if fInfo := f.fieldsInfo(); fInfo != nil {
		info = fInfo[tfName]
	}

	schemaField, ok := f.asObj().GetOk(tfName)
	if !ok {
		return MissingElem{}
	}

	// If we are looking at a resource, we return a FullObject
	if _, ok := tfshimutil.CastToTypeObject(schemaField); ok {
		return FullObject{
			schema: schemaField,
			info:   info,
		}
	}

	// Since we are not looking at a resource, we return a FullSchema.
	return FullSchema{
		schema: schemaField,
		info:   info,
	}
}

func (f FullSchema) Elem() FullElem {
	var elemInfo *SchemaInfo
	if f.info != nil {
		elemInfo = f.info.Elem
	}
	switch elem := f.schema.Elem().(type) {
	case shim.Schema:
		return FullSchema{
			schema: elem,
			info:   elemInfo,
		}
	case shim.Resource:
		f := FullObject{
			schema: (&schema.Schema{Elem: elem, Type: shim.TypeMap}).Shim(),
			info:   elemInfo,
		}
		f.asObj()
		return f
	default:
		return MissingElem{}
	}
}
