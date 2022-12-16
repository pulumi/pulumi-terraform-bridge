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
	"fmt"
	"reflect"

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Attr type works around not being able to link to fwschema.Attribute from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
//
// Most methods from fwschema.Attribute have simple signatures and are copied into attrLike interface. Casting to
// attrLike exposes these methods.
//
// GetAttributes method is special since it returns a NestedAttributes interface that is also internal and cannot be
// linked to. Instead, NestedAttriutes information is recorded in a dedicated new field.
type Attr struct {
	AttrLike
	Nested map[string]Attr
}

type AttrLike interface {
	FrameworkType() pfattr.Type
	IsComputed() bool
	IsOptional() bool
	IsRequired() bool
	IsSensitive() bool
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string
}

func AttributeAtTerraformPath(schema *tfsdk.Schema, path *tftypes.AttributePath) (Attr, error) {
	res, remaining, err := tftypes.WalkAttributePath(*schema, path)
	if err != nil {
		return Attr{}, fmt.Errorf("%v still remains in the path: %w", remaining, err)
	}
	switch r := res.(type) {
	case tfsdk.Attribute:
		m := SchemaToAttrMap(&tfsdk.Schema{
			Attributes: map[string]tfsdk.Attribute{
				"x": r,
			},
		})
		return m["x"], nil
	default:
		return Attr{}, fmt.Errorf("Expected a Block but found %s at path %s",
			reflect.TypeOf(r), path)
	}
}

func SchemaToAttrMap(schema *tfsdk.Schema) map[string]Attr {
	if schema.GetAttributes() == nil || len(schema.GetAttributes()) == 0 {
		return map[string]Attr{}
	}

	// unable to reference fwschema.Attribute type directly, use GetAttriutes to hijack this type and get a
	// collection variable (happens to be a map) that lets us track multiple instances of this type
	queue := schema.GetAttributes()

	// only the datastructure is needed, not the content, so clear all content here
	for k := range queue {
		delete(queue, k)
	}

	// pair queue with dests to record pending work; if queue[k] is a fwschema.Attribute to convert, then dests[k]
	// records where the result of conversion should be stored:
	//
	//     dests[k].toMap[dests[k].key] = convert(queue[k])
	type dest = struct {
		toMap map[string]Attr
		key   string
	}
	dests := map[string]dest{}

	jobCounter := 0

	// queue up converting schema.GetAttributes() into finalMap
	finalMap := map[string]Attr{}
	for k, v := range schema.GetAttributes() {
		job := fmt.Sprintf("%d", jobCounter)
		jobCounter++
		queue[job] = v
		dests[job] = dest{toMap: finalMap, key: k}
	}

	// keep converting until work queue is empty
	for len(queue) > 0 {
		job, inAttr := pop(queue)
		attrDest := popAt(dests, job)

		// outAttr := convert(inAttr)
		outAttr := Attr{AttrLike: inAttr}
		if nested := inAttr.GetAttributes(); nested != nil && nested.GetAttributes() != nil {
			outAttr.Nested = map[string]Attr{}
			for k, v := range nested.GetAttributes() {
				// schedule outAttr.nested[k] = convert(v)
				job := fmt.Sprintf("%d", jobCounter)
				jobCounter++
				queue[job] = v
				dests[job] = dest{toMap: outAttr.Nested, key: k}
			}
		}
		attrDest.toMap[attrDest.key] = outAttr
	}

	return finalMap
}
