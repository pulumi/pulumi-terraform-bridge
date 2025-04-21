// Copyright 2016-2025, Pulumi Corporation.
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

package tfbridgetests

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
)

func hclWriteResource(
	t *testing.T,
	out io.Writer,
	resourceType string,
	resourceSchema resource.Resource,
	resourceName string,
	resourceConfig tftypes.Value,
) {
	rs := getResourceSchema(t, resourceSchema)
	v := convertTValueToCtyValue(t, resourceConfig)
	err := hclwrite.WriteResource(out, &hclSchema{rs}, resourceType, resourceName, v.AsValueMap())
	require.NoErrorf(t, err, "Failed to write HCL")
}

func convertTValueToCtyValue(t *testing.T, value tftypes.Value) cty.Value {
	dv, err := tfprotov6.NewDynamicValue(value.Type(), value)
	require.NoErrorf(t, err, "tfprotov6.NewDynamicValue failed")

	var ctyType cty.Type

	typeBytes, err := json.Marshal(value.Type())
	require.NoErrorf(t, err, "json.Marshal(value.Type()) failed")

	err = json.Unmarshal(typeBytes, &ctyType)
	require.NoErrorf(t, err, "json.Unmarshal() failed to recover a cty.Type")

	v, err := msgpack.Unmarshal(dv.MsgPack, ctyType)
	require.NoErrorf(t, err, "msgpack.Unmarshal() failed to recover a cty.Value")

	return v
}

func getResourceSchema(t *testing.T, res resource.Resource) schema.Schema {
	resp := &resource.SchemaResponse{}
	res.Schema(context.Background(), resource.SchemaRequest{}, resp)
	for _, d := range resp.Diagnostics {
		t.Logf("res.Schema(): %v", d)
	}
	require.Falsef(t, resp.Diagnostics.HasError(), "res.Schema() returned error diagnostics")
	return resp.Schema
}

type hclSchema struct {
	s schema.Schema
}

var _ hclwrite.ShimHCLSchema = (*hclSchema)(nil)

func (h *hclSchema) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	m := make(map[string]hclwrite.ShimHCLAttribute, len(h.s.Attributes))
	for k := range h.s.Attributes {
		m[k] = hclwrite.ShimHCLAttribute{}
	}
	return m
}

func (h *hclSchema) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	blocks := h.s.GetBlocks()
	m := make(map[string]hclwrite.ShimHCLBlock, len(blocks))
	for k, b := range blocks {
		m[k] = &hclBlock{Block: b}
	}
	return m
}

type hclBlock struct {
	schema.Block
}

var _ hclwrite.ShimHCLBlock = (*hclBlock)(nil)

func (b *hclBlock) GetNestingMode() hclwrite.Nesting {
	switch b.Block.GetNestingMode() {
	case 1: //list
		return hclwrite.NestingList
	case 2: //set
		return hclwrite.NestingSet
	case 3: //single
		return hclwrite.NestingSingle
	default:
		return hclwrite.NestingInvalid
	}
}

func (b *hclBlock) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	attrs := b.Block.GetNestedObject().GetAttributes()
	m := make(map[string]hclwrite.ShimHCLAttribute, len(attrs))
	for k := range attrs {
		m[k] = hclwrite.ShimHCLAttribute{}
	}
	return m
}

func (b *hclBlock) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	blocks := b.Block.GetNestedObject().GetBlocks()
	m := make(map[string]hclwrite.ShimHCLBlock, len(blocks))
	for k, b := range blocks {
		m[k] = &hclBlock{Block: b}
	}
	return m
}
