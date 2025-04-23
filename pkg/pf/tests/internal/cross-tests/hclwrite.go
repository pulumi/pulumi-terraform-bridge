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

package crosstests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
)

func hclWriteResource(
	t *testing.T,
	out io.Writer,
	resourceType string,
	resourceSchema resource.Resource,
	resourceName string,
	resourceConfig cty.Value,
) {
	rs := getResourceSchema(t, resourceSchema)
	vs := resourceConfig.AsValueMap()
	err := hclwrite.WriteResource(out, hclSchemaPFResource(rs), resourceType, resourceName, vs)
	require.NoErrorf(t, err, "Failed to write HCL")
}

func convertTValueToCtyValue(value tftypes.Value) (cty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(value.Type(), value)
	if err != nil {
		return cty.NilVal, fmt.Errorf("tfprotov6.NewDynamicValue failed: %w", err)
	}

	var ctyType cty.Type

	typeBytes, err := json.Marshal(value.Type())
	if err != nil {
		return cty.NilVal, fmt.Errorf("json.Marshal(value.Type()) failed: %w", err)
	}

	if err = json.Unmarshal(typeBytes, &ctyType); err != nil {
		return cty.NilVal, fmt.Errorf("json.Unmarshal() failed to recover a cty.Type: %w", err)
	}

	v, err := msgpack.Unmarshal(dv.MsgPack, ctyType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("msgpack.Unmarshal() failed to recover a cty.Value: %w", err)
	}
	return v, nil
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
