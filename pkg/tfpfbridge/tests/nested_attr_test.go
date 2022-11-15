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

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testprovider"
)

func TestNestedType(t *testing.T) {
	ctx := context.TODO()
	info := testprovider.SyntheticTestBridgeProvider()
	res, err := pfutils.GatherResources(ctx, info.P())
	require.NoError(t, err)
	testresTypeName := res.All()[0]

	testresType := res.Schema(testresTypeName).Type().TerraformType(ctx)

	obj := testresType.(tftypes.Object).AttributeTypes["services"].(tftypes.List).ElementType.(tftypes.Object)
	assert.True(t, obj.AttributeTypes["protocol"].Is(tftypes.String))
}

func TestNestedOptionals(t *testing.T) {
	t.Skip("Skipped as possible issue with terraform-plugin-framework")
	ctx := context.TODO()
	nestedOptional := tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
		"nestedOptional": {
			Optional: true,
			Type:     types.StringType,
		},
	})
	assert.True(t, nestedOptional.GetAttributes()["nestedOptional"].IsOptional())
	// Projecting to tftypes.Object seems to forget the optional-ity of nestedOptional.
	assert.Contains(t, nestedOptional.Type().TerraformType(ctx).(tftypes.Object).OptionalAttributes, "nestedOptional")
}
