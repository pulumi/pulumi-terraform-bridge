// Copyright 2016-2019, Pulumi Corporation.
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

package provider

import (
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/terraform/states"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestStructpbToCtyObject(t *testing.T) {
	input := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"bucket": {
				Kind: &structpb.Value_StringValue{
					StringValue: "test",
				},
			},
			"key": {
				Kind: &structpb.Value_StringValue{
					StringValue: "test2",
				},
			},
			"encrypt": {
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			},
		},
	}

	expected := cty.ObjectVal(map[string]cty.Value{
		"encrypt": cty.BoolVal(true),
		"key":     cty.StringVal("test2"),
		"bucket":  cty.StringVal("test"),
	})

	actual, err := structpbToCtyObject(input)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPulumiNameToTfName(t *testing.T) {
	require.Equal(t, "camel_case_name", pulumiNameToTfName("camelCaseName"))
	require.Equal(t, "http_auth", pulumiNameToTfName("httpAuth"))
	require.Equal(t, "region", pulumiNameToTfName("region"))
}

func TestStructpbNamesPulumiToTerraform(t *testing.T) {
	input := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"bucket": {
				Kind: &structpb.Value_StringValue{
					StringValue: "test",
				},
			},
			"httpAuth": {
				Kind: &structpb.Value_StringValue{
					StringValue: "test2",
				},
			},
			"useHttps": {
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			},
		},
	}

	expected := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"bucket": {
				Kind: &structpb.Value_StringValue{
					StringValue: "test",
				},
			},
			"http_auth": {
				Kind: &structpb.Value_StringValue{
					StringValue: "test2",
				},
			},
			"use_https": {
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			},
		},
	}

	require.Equal(t, expected, structpbNamesPulumiToTerraform(input))
}

func TestOutputsToStructpb(t *testing.T) {
	input := map[string]*states.OutputValue{
		"string_val": {
			Value: cty.StringVal("A String"),
		},
		"list_val": {
			Value: cty.ListVal([]cty.Value{
				cty.StringVal("String 1"),
				cty.StringVal("String 2"),
			}),
		},
		"map_val": {
			Value: cty.MapVal(map[string]cty.Value{
				"key1": cty.StringVal("Value 1"),
				"key2": cty.StringVal("Value 2"),
			}),
		},
	}

	expected := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"string_val": {
				Kind: &structpb.Value_StringValue{
					StringValue: "A String",
				},
			},
			"list_val": {
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{
								Kind: &structpb.Value_StringValue{
									StringValue: "String 1",
								},
							},
							{
								Kind: &structpb.Value_StringValue{
									StringValue: "String 2",
								},
							},
						},
					},
				},
			},
			"map_val": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"key1": {
								Kind: &structpb.Value_StringValue{
									StringValue: "Value 1",
								},
							},
							"key2": {
								Kind: &structpb.Value_StringValue{
									StringValue: "Value 2",
								},
							},
						},
					},
				},
			},
		},
	}

	outputs, err := outputsToStructpb(input)
	require.NoError(t, err)
	require.Equal(t, expected, outputs)
}
