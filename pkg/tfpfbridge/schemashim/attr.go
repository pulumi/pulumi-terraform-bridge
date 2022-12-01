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

package schemashim

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
)

// attr type works around not being able to link to fwschema.Attribute from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
//
// Most methods from fwschema.Attribute have simple signatures and are copied into attrLike interface. Casting to
// attrLike exposes these methods.
//
// GetAttributes method is special since it returns a NestedAttributes interface that is also internal and cannot be
// linked to. Instead, NestedAttriutes information is recorded in a dedicated new field.
type attr struct {
	attrLike
	nested map[string]attr
}

type attrLike interface {
	FrameworkType() pfattr.Type
	IsComputed() bool
	IsOptional() bool
	IsRequired() bool
	IsSensitive() bool
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string
}

func schemaToAttrMap(schema *tfsdk.Schema) map[string]attr {
	if schema.GetAttributes() == nil || len(schema.GetAttributes()) == 0 {
		return map[string]attr{}
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
		toMap map[string]attr
		key   string
	}
	dests := map[string]dest{}

	jobCounter := 0

	// queue up converting schema.GetAttributes() into finalMap
	finalMap := map[string]attr{}
	for k, v := range schema.GetAttributes() {
		job := fmt.Sprintf("%d", jobCounter)
		jobCounter++
		queue[job] = v
		dests[job] = dest{toMap: finalMap, key: k}
	}

	// keep converting until work queue is empty
	for len(queue) > 0 {

		// pick and pop a random job from the queue
		var job string
		for j := range queue {
			job = j
			break
		}

		inAttr, attrDest := queue[job], dests[job]

		delete(queue, job)
		delete(dests, job)

		// outAttr := convert(inAttr)
		outAttr := attr{attrLike: inAttr}
		if nested := inAttr.GetAttributes(); nested != nil && nested.GetAttributes() != nil {
			outAttr.nested = map[string]attr{}
			for k, v := range nested.GetAttributes() {
				// schedule outAttr.nested[k] = convert(v)
				job := fmt.Sprintf("%d", jobCounter)
				jobCounter++
				queue[job] = v
				dests[job] = dest{toMap: outAttr.nested, key: k}
			}
		}
		attrDest.toMap[attrDest.key] = outAttr
	}

	return finalMap
}
