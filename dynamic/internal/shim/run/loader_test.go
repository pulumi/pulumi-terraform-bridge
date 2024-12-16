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

package run

import (
	"context"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Integration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping integration test during -short")
	}
}

// The use of t.Setenv makes it necessary to disable t.Parallel() and skip the paralleltest linter rule.
func TestLoadProvider(t *testing.T) { //nolint:paralleltest
	if runtime.GOOS != "windows" {
		// Do not cache during the test. This does not seem to work on Windows correctly due to temp dir cleanup
		// issues, therefore when running on Windows beware that the test may over-optimistically pass against
		// a cached result from the previous run.
		t.Setenv(envPluginCache, t.TempDir())
	}

	t.Run("registry", func(t *testing.T) {
		Integration(t)
		ctx := context.Background()

		p, err := NamedProvider(ctx, "hashicorp/tls", "<4.0.5,>4.0.3")
		require.NoError(t, err)

		require.Equal(t, "4.0.4", p.Version())

		resp, err := p.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
		require.NoError(t, err)

		assert.Equal(t, &tfprotov6.Schema{Block: &tfprotov6.SchemaBlock{
			Description:     "Provider configuration",
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Attributes:      []*tfprotov6.SchemaAttribute{},
			BlockTypes: []*tfprotov6.SchemaNestedBlock{{
				TypeName: "proxy",
				Block: &tfprotov6.SchemaBlock{
					Description:     "Proxy used by resources and data sources that connect to external endpoints.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Attributes: []*tfprotov6.SchemaAttribute{
						{
							Name: "from_env",
							Type: tftypes.Bool,
							//nolint:lll
							Description:     "When `true` the provider will discover the proxy configuration from environment variables. This is based upon [`http.ProxyFromEnvironment`](https://pkg.go.dev/net/http#ProxyFromEnvironment) and it supports the same environment variables (default: `true`).",
							Optional:        true,
							Computed:        true,
							DescriptionKind: tfprotov6.StringKindMarkdown,
						},
						{
							Name:            "password",
							Type:            tftypes.String,
							Description:     "Password used for Basic authentication against the Proxy.",
							DescriptionKind: tfprotov6.StringKindMarkdown,
							Optional:        true,
							Sensitive:       true,
						},
						{
							Name:            "url",
							Type:            tftypes.String,
							Description:     "URL used to connect to the Proxy. Accepted schemes are: `http`, `https`, `socks5`. ",
							DescriptionKind: tfprotov6.StringKindMarkdown,
							Optional:        true,
						},
						{
							Name:            "username",
							Type:            tftypes.String,
							Description:     "Username (or Token) used for Basic authentication against the Proxy.",
							DescriptionKind: tfprotov6.StringKindMarkdown,
							Optional:        true,
						},
					},
					BlockTypes: []*tfprotov6.SchemaNestedBlock{},
				},
				Nesting:  tfprotov6.SchemaNestedBlockNestingModeList,
				MaxItems: 1,
			}},
		}}, resp.Provider)
	})
}
