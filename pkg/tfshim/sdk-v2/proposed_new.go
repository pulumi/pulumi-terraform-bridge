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

package sdkv2

import (
	"context"

	hcty "github.com/hashicorp/go-cty/cty"
	hctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/tf/configs/configschema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/tf/plans/objchange"
)

func simpleDiff(
	ctx context.Context,
	res *schema.Resource,
	s *terraform.InstanceState,
	c *terraform.ResourceConfig,
	rawConfigVal hcty.Value,
	meta interface{},
) (*terraform.InstanceDiff, error) {
	b, err := configschemaBlock(res)
	if err != nil {
		return nil, err
	}

	priorStateVal, err := s.AttrsAsObjectValue(res.CoreConfigSchema().ImpliedType())
	if err != nil {
		return nil, err
	}

	proposedNewStateVal, err := proposedNew(b, priorStateVal, rawConfigVal)
	if err != nil {
		return nil, err
	}

	config := terraform.NewResourceConfigShimmed(proposedNewStateVal, res.CoreConfigSchema())
	return res.SimpleDiff(ctx, s, config, meta)
}

func proposedNew(schema *configschema.Block, prior, config hcty.Value) (hcty.Value, error) {
	priorC, err := hcty2cty(prior)
	if err != nil {
		return hcty.False, err
	}
	configC, err := hcty2cty(prior)
	if err != nil {
		return hcty.False, err
	}
	return cty2hcty(objchange.ProposedNew(schema, priorC, configC))
}

func htype2ctype(t hcty.Type) (cty.Type, error) {
	if t.Equals(hcty.NilType) {
		return cty.NilType, nil
	}
	typeJSON, err := hctyjson.MarshalType(t)
	if err != nil {
		return cty.Bool, err
	}
	return ctyjson.UnmarshalType(typeJSON)
}

func hcty2cty(v hcty.Value) (cty.Value, error) {
	if v.Equals(hcty.NilVal).True() {
		return cty.NilVal, nil
	}
	typ, err := htype2ctype(v.Type())
	if err != nil {
		return cty.False, err
	}
	valueJSON, err := hctyjson.Marshal(v, v.Type())
	if err != nil {
		return cty.False, err
	}
	return ctyjson.Unmarshal(valueJSON, typ)
}

func cty2hcty(v cty.Value) (hcty.Value, error) {
	if v.Equals(cty.NilVal).True() {
		return hcty.NilVal, nil
	}

	typeJSON, err := ctyjson.MarshalType(v.Type())
	if err != nil {
		return hcty.False, err
	}
	valueJSON, err := ctyjson.Marshal(v, v.Type())
	if err != nil {
		return hcty.False, err
	}
	typ, err := hctyjson.UnmarshalType(typeJSON)
	if err != nil {
		return hcty.False, err
	}
	return hctyjson.Unmarshal(valueJSON, typ)
}

func configschemaBlock(res *schema.Resource) (*configschema.Block, error) {
	schema := res.CoreConfigSchema()

	block := &configschema.Block{
		Attributes:      map[string]*configschema.Attribute{},
		BlockTypes:      map[string]*configschema.NestedBlock{},
		Description:     schema.Description,
		DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
		Deprecated:      schema.Deprecated,
	}

	for name, a := range schema.Attributes {
		t, err := htype2ctype(a.Type)
		if err != nil {
			return nil, err
		}
		block.Attributes[name] = &configschema.Attribute{
			Type:            t,
			Description:     a.Description,
			DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
			Required:        a.Required,
			Optional:        a.Optional,
			Computed:        a.Computed,
			Sensitive:       a.Sensitive,
			Deprecated:      a.Deprecated,
		}
	}

	queue := newQueue(schema.BlockTypes)

	destinations := map[interface{}]map[string]*configschema.NestedBlock{}
	for _, b := range schema.BlockTypes {
		destinations[b] = block.BlockTypes
	}

	for !queue.empty() {
		name, b := queue.dequeue()

		nested := configschema.Block{
			Attributes:      map[string]*configschema.Attribute{},
			BlockTypes:      map[string]*configschema.NestedBlock{},
			Description:     b.Description,
			DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
			Deprecated:      b.Deprecated,
		}

		for name, a := range b.Attributes {
			t, err := htype2ctype(a.Type)
			if err != nil {
				return nil, err
			}
			nested.Attributes[name] = &configschema.Attribute{
				Type:            t,
				Description:     a.Description,
				DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
				Required:        a.Required,
				Optional:        a.Optional,
				Computed:        a.Computed,
				Sensitive:       a.Sensitive,
				Deprecated:      a.Deprecated,
			}
		}

		for name, nb := range b.BlockTypes {
			destinations[nb] = nested.BlockTypes
			queue.enqueue(name, nb)
		}

		destinations[b][name] = &configschema.NestedBlock{
			Block:    nested,
			Nesting:  configschema.NestingMode(int(b.Nesting)),
			MinItems: b.MinItems,
			MaxItems: b.MaxItems,
		}
	}

	return block, nil
}

type queue[T any] struct {
	elems []struct {
		key   string
		value *T
	}
}

func (q *queue[T]) dequeue() (string, *T) {
	k := q.elems[0].key
	v := q.elems[0].value
	q.elems = q.elems[1:]
	return k, v
}

func (q *queue[T]) empty() bool {
	return len(q.elems) == 0
}

func (q *queue[T]) enqueue(key string, value *T) {
	q.elems = append(q.elems, struct {
		key   string
		value *T
	}{key: key, value: value})
}

func newQueue[T any](starter map[string]*T) *queue[T] {
	q := &queue[T]{}
	for k, v := range starter {
		q.enqueue(k, v)
	}
	return q
}
