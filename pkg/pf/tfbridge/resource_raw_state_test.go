// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package tfbridge

import (
	"context"
	"encoding/json"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func Test_parseRawResourceState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sch := schemashim.NewSchemaMap(
		pfutils.FromResourceSchema(
			rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"s": rschema.StringAttribute{
						Optional: true,
					},
				},
			},
		),
	)

	st, err := parseRawResourceState(
		ctx, &tfbridge.ResourceInfo{}, sch, "test", resource.ID("id"), 0,
		resource.PropertyMap{"s": resource.NewStringProperty("s1")})
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(*st, &m))

	autogold.Expect(map[string]any{"id": "id", "s": "s1"}).Equal(t, m)
}
