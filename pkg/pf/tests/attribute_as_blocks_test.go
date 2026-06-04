// Copyright 2026, Pulumi Corporation.
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

package tfbridgetests

import (
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

// An SDKv2 block declared with ConfigMode: SchemaConfigModeAttr ("attribute-as-blocks") is not
// serialized as a NestedBlock over the Terraform plugin wire protocol. Instead it becomes an
// Attribute whose type is a List<Object>. The per-field Optional/Required information that exists
// in the SDKv2 Go source is *not* encoded into that object type: the wire type carries no
// OptionalAttributes.
func TestSDKv2AttributeAsBlocksOptionalityLostOverWire(t *testing.T) {
	t.Parallel()

	sdkProv := &sdkschema.Provider{
		ResourcesMap: map[string]*sdkschema.Resource{
			"testprovider_sg": {
				Schema: map[string]*sdkschema.Schema{
					"ingress": {
						Type:       sdkschema.TypeList,
						Optional:   true,
						ConfigMode: sdkschema.SchemaConfigModeAttr,
						Elem: &sdkschema.Resource{
							Schema: map[string]*sdkschema.Schema{
								"from_port": {Type: sdkschema.TypeInt, Required: true},
								"to_port":   {Type: sdkschema.TypeInt, Required: true},
								"protocol":  {Type: sdkschema.TypeString, Required: true},
								// description is optional in the SDKv2 source. This is the field
								// whose optionality is lost over the wire.
								"description": {Type: sdkschema.TypeString, Optional: true},
								"cidr_blocks": {
									Type:     sdkschema.TypeList,
									Optional: true,
									Elem:     &sdkschema.Schema{Type: sdkschema.TypeString},
								},
							},
						},
					},
				},
			},
		},
	}

	v6server, err := tf5to6server.UpgradeServer(t.Context(), func() tfprotov5.ProviderServer {
		return sdkProv.GRPCProvider()
	})
	require.NoError(t, err)

	// Part 1: show the root cause directly on the wire schema.
	schemaResp, err := v6server.GetProviderSchema(t.Context(), &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	var ingressAttr *tfprotov6.SchemaAttribute
	for _, a := range schemaResp.ResourceSchemas["testprovider_sg"].Block.Attributes {
		if a.Name == "ingress" {
			ingressAttr = a
		}
	}
	require.NotNil(t, ingressAttr, "ConfigMode attr block should appear as an attribute, not a block")
	require.Nil(t, ingressAttr.NestedType, "attribute-as-blocks is typed, not a NestedType, over the wire")

	listType, ok := ingressAttr.Type.(tftypes.List)
	require.True(t, ok, "ingress should be a List, got %v", ingressAttr.Type)
	objType, ok := listType.ElementType.(tftypes.Object)
	require.True(t, ok, "ingress element should be an Object, got %v", listType.ElementType)

	// The object carries all five field types but no per-field optionality: this is the
	// information the bridge cannot recover.
	require.Len(t, objType.AttributeTypes, 5)
	require.Empty(t, objType.OptionalAttributes,
		"SDKv2 does not encode per-field optionality for attribute-as-blocks over the wire")

	// Part 2: show that the dynamic bridge defaults to optional rather than required.
	info := tfbridge0.ProviderInfo{
		Name:         "testprovider",
		Version:      "0.0.1",
		P:            proto.New(t.Context(), v6server),
		MetadataInfo: &tfbridge0.MetadataInfo{},
		Resources: map[string]*tfbridge0.ResourceInfo{
			"testprovider_sg": {Tok: "testprovider:index:Sg"},
		},
	}

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	spec, err := tfgen.GenerateSchema(info, nilSink)
	require.NoError(t, err)

	require.Empty(t, spec.Types["testprovider:index/SgIngress:SgIngress"].Required)
}
