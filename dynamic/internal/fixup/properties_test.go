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

package fixup_test

import (
	"testing"

	_ "github.com/hexops/autogold/v2" // autogold registers a flag for -update
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/dynamic/internal/fixup"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestFixMissingID(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_res": (&schema.Resource{
					Schema: schema.SchemaMap{
						"some_property": (&schema.Schema{
							Type: shim.TypeString,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}

	err := fixup.Default(&p)
	require.NoError(t, err)
	assert.NotNil(t, p.Resources["test_res"].ComputeID)
}

func TestFixURNProperty(t *testing.T) {
	t.Parallel()
	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_res": (&schema.Resource{
					Schema: schema.SchemaMap{
						"urn": (&schema.Schema{
							Type: shim.TypeString,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}

	err := fixup.Default(&p)
	require.NoError(t, err)
	assert.Equal(t, &info.Schema{Name: "testUrn"}, p.Resources["test_res"].Fields["urn"])
}

func TestFixProviderResourceName(t *testing.T) {
	t.Parallel()
	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_provider": (&schema.Resource{
					Schema: schema.SchemaMap{
						"id": (&schema.Schema{
							Type:     shim.TypeString,
							Computed: true,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}

	err := fixup.Default(&p)
	require.NoError(t, err)
	assert.Equal(t, tokens.Type("test:index/testProvider:TestProvider"), p.Resources["test_provider"].Tok)
}
