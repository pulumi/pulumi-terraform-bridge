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

package tfbridge

import (
	"context"
	"fmt"
	"testing"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestDeconflict(t *testing.T) {
	ctx := context.Background()

	ctx = logging.InitLogging(ctx, logging.LogOptions{})
	schema := &schema.SchemaMap{
		"availability_zone": (&schema.Schema{
			Type:          shim.TypeString,
			Optional:      true,
			Computed:      true,
			ForceNew:      true,
			ConflictsWith: []string{"availability_zone_id"},
		}).Shim(),
		"availability_zone_id": (&schema.Schema{
			Type:          shim.TypeString,
			Optional:      true,
			Computed:      true,
			ForceNew:      true,
			ConflictsWith: []string{"availability_zone"},
		}).Shim(),
	}

	type testCase struct {
		inputs   resource.PropertyMap
		expected resource.PropertyMap
	}

	testCases := []testCase{
		{
			inputs: resource.PropertyMap{
				"availabilityZone":   resource.NewStringProperty("us-east-1c"),
				"availabilityZoneId": resource.NewStringProperty("use1-az1"),
			},
			expected: resource.PropertyMap{
				"availabilityZone":   resource.NewStringProperty("us-east-1c"),
				"availabilityZoneId": resource.NewNullProperty(),
			},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			actual := deconflict(ctx, schema, nil, tc.inputs)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestParseConflictsWith(t *testing.T) {
	cw := "capacity_reservation_specification.0.capacity_reservation_target.0.capacity_reservation_id"
	actual := parseConflictsWith(cw)
	expect := "capacity_reservation_specification.$.capacity_reservation_target.$.capacity_reservation_id"
	require.Equal(t, expect, actual.MustEncodeSchemaPath())
}
