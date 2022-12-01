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

	// fresh generates unique job IDs
	counter := 0
	fresh := func() string {
		counter++
		return fmt.Sprintf("%d", counter)
	}

	// unable to reference fwschema.Attribute type directly, use GetAttriutes to hijack this type and get a
	// collection variable (happens to be a map) that lets us track multiple instances of this type
	queue := schema.GetAttributes()

	// returns a random first key from queue
	next := func() string {
		for k := range queue {
			return k
		}
		return ""
	}

	// clear queue
	for len(queue) > 0 {
		delete(queue, next())
	}

	// pair queue with dests to record pending work; if queue[k] is a fwschema.Attribute to convert, then dests[k]
	// records where the result of conversion should be stored:
	//
	//     dests[k].toMap[dests[k].key] = conv(queue[k])
	type dest = struct {
		toMap map[string]attr
		key   string
	}
	dests := map[string]dest{}

	// queue up converting schema.GetAttributes() into finalMap
	finalMap := map[string]attr{}
	for k, v := range schema.GetAttributes() {
		job := fresh()
		queue[job] = v
		dests[job] = dest{toMap: finalMap, key: k}
	}

	// keep converting until work queue is empty
	for len(queue) > 0 {
		// pop into a, d
		k := next()
		a := queue[k]
		delete(queue, k)
		d := dests[k]
		delete(dests, k)

		// r := convert(a)
		r := attr{attrLike: a}
		if n := a.GetAttributes(); n != nil && n.GetAttributes() != nil {
			r.nested = map[string]attr{}
			for k, v := range n.GetAttributes() {
				// schedule r.nested[k] = convert(v)
				job := fresh()
				queue[job] = v
				dests[job] = dest{toMap: r.nested, key: k}
			}
		}
		d.toMap[d.key] = r
	}

	return finalMap
}
