// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/opentofu/opentofu/shim/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/dynamic/parameterize"
)

func TestInferResourcePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		tfNames        []string
		expectedPrefix string
	}{
		{
			name: "divergent-prefix",
			tfNames: []string{
				"prefix_res1",
				"prefix_res2",
			},
			expectedPrefix: "prefix",
		},
		{
			name: "expected-prefix",
			tfNames: []string{
				"test_res1",
				"test_res2",
			},
			expectedPrefix: "test",
		},
		{
			name: "ambiguous-prefix",
			tfNames: []string{
				"test_res1",
				"test2_res",
			},
			// We expect "test" because that is the .Name of the provider.
			expectedPrefix: "test",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resourceSchemas := make(map[string]*tfprotov6.Schema, len(tt.tfNames))
			for _, tf := range tt.tfNames {
				resourceSchemas[tf] = &tfprotov6.Schema{Block: &tfprotov6.SchemaBlock{}}
			}

			info, err := providerInfo(context.Background(), schemaOnlyProvider{
				name:    "test",
				version: "1.0.0",
				schema: &tfprotov6.GetProviderSchemaResponse{
					ResourceSchemas: resourceSchemas,
				},
			}, parameterize.Value{})
			require.NoError(t, err)

			assert.Equal(t, tt.expectedPrefix, info.GetResourcePrefix())
		})
	}
}

type schemaOnlyProvider struct {
	run.Provider
	name, url, version string

	schema *tfprotov6.GetProviderSchemaResponse
}

func (s schemaOnlyProvider) Name() string    { return s.name }
func (s schemaOnlyProvider) URL() string     { return s.url }
func (s schemaOnlyProvider) Version() string { return s.version }

func (s schemaOnlyProvider) GetProviderSchema(
	_ context.Context, req *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	return s.schema, nil
}
