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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

func TestDeconflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctx = logging.InitLogging(ctx, logging.LogOptions{})

	type testCase struct {
		name      string
		schemaMap shim.SchemaMap
		inputs    resource.PropertyMap
		expected  resource.PropertyMap
	}

	testCases := []testCase{
		{
			// The base case is not modifying inputs when there is no conflict.
			name: "aws-subnet-no-conflict",
			schemaMap: &schema.SchemaMap{
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
			},
			inputs: resource.PropertyMap{
				"availabilityZone": resource.NewStringProperty("us-east-1c"),
			},
			expected: resource.PropertyMap{
				"availabilityZone": resource.NewStringProperty("us-east-1c"),
			},
		},
		{
			// Typical example of ConflictsWith, highly voted AWS resource excerpt here.
			name: "aws-subnet",
			schemaMap: &schema.SchemaMap{
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
			},
			inputs: resource.PropertyMap{
				"availabilityZone":   resource.NewStringProperty("us-east-1c"),
				"availabilityZoneId": resource.NewStringProperty("use1-az1"),
			},
			expected: resource.PropertyMap{
				"availabilityZone": resource.NewStringProperty("us-east-1c"),
			},
		},
		{
			// Usually ConflictsWith are bi-directional but this AWS resource has it one-directional.
			name: "aws-ipam-pool-cidr",
			schemaMap: &schema.SchemaMap{
				"cidr": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
					ForceNew: true,
					Computed: true,
				}).Shim(),
				"netmask_length": (&schema.Schema{
					Type:          shim.TypeInt,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"cidr"},
				}).Shim(),
			},
			inputs: resource.PropertyMap{
				"cidr":          resource.NewStringProperty("192.0.2.0/24"),
				"netmaskLength": resource.NewNumberProperty(24),
			},
			expected: resource.PropertyMap{
				"cidr": resource.NewStringProperty("192.0.2.0/24"),
			},
		},
	}

	// aws_autoscaling_group has three-way partial triangle of ConflictsWith.
	asgSchema := &schema.SchemaMap{
		"load_balancers": (&schema.Schema{
			Type:          shim.TypeSet,
			Optional:      true,
			Computed:      true,
			Elem:          (&schema.Schema{Type: shim.TypeString}).Shim(),
			ConflictsWith: []string{"traffic_source"},
		}).Shim(),
		"target_group_arns": (&schema.Schema{
			Type:          shim.TypeSet,
			Optional:      true,
			Computed:      true,
			Elem:          (&schema.Schema{Type: shim.TypeString}).Shim(),
			ConflictsWith: []string{"traffic_source"},
		}).Shim(),
		"traffic_source": (&schema.Schema{
			Type:     shim.TypeSet,
			Optional: true,
			Computed: true,
			Elem: (&schema.Resource{
				Schema: &schema.SchemaMap{
					"identifier": (&schema.Schema{
						Type:     shim.TypeString,
						Required: true,
					}).Shim(),
				},
			}).Shim(),
			ConflictsWith: []string{"load_balancers", "target_group_arns"},
		}).Shim(),
	}

	testCases = append(testCases, []testCase{
		{
			name:      "aws-autoscaling-group-keep",
			schemaMap: asgSchema,
			inputs: resource.PropertyMap{
				"loadBalancers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("lb1"),
					resource.NewStringProperty("lb2"),
				}),
				"targetGroupArns": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("arn1"),
					resource.NewStringProperty("arn2"),
				}),
			},
			expected: resource.PropertyMap{
				"loadBalancers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("lb1"),
					resource.NewStringProperty("lb2"),
				}),
				"targetGroupArns": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("arn1"),
					resource.NewStringProperty("arn2"),
				}),
			},
		},
		{
			name:      "aws-autoscaling-group-drop-traffic-sources",
			schemaMap: asgSchema,
			inputs: resource.PropertyMap{
				"loadBalancers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("lb1"),
					resource.NewStringProperty("lb2"),
				}),
				"targetGroupArns": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("arn1"),
					resource.NewStringProperty("arn2"),
				}),
				"trafficSources": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"identifier": resource.NewStringProperty("id1"),
					}),
				}),
			},
			expected: resource.PropertyMap{
				"loadBalancers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("lb1"),
					resource.NewStringProperty("lb2"),
				}),
				"targetGroupArns": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("arn1"),
					resource.NewStringProperty("arn2"),
				}),
			},
		},
	}...)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual := deconflict(ctx, tc.schemaMap, nil, tc.inputs)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestParseConflictsWith(t *testing.T) {
	t.Parallel()
	cw := "capacity_reservation_specification.0.capacity_reservation_target.0.capacity_reservation_id"
	actual := parseConflictsWith(cw)
	expect := "capacity_reservation_specification.$.capacity_reservation_target.$.capacity_reservation_id"
	require.Equal(t, expect, actual.MustEncodeSchemaPath())
}
