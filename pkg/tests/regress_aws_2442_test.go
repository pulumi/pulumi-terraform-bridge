// Copyright 2016-2023, Pulumi Corporation.
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

package tests

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"hash/crc32"
)

func TestRegressAws2442(t *testing.T) {
	ctx := context.Background()

	stringHashcode := func(s string) int {
		v := int(crc32.ChecksumIEEE([]byte(s)))
		if v >= 0 {
			return v
		}
		if -v >= 0 {
			return -v
		}
		// v == MinInt
		return 0
	}

	hashes := map[int]string{}

	resourceParameterHash := func(v interface{}) int {
		var buf bytes.Buffer
		m := v.(map[string]interface{})
		// Store the value as a lower case string, to match how we store them in FlattenParameters
		name := strings.ToLower(m["name"].(string))
		buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(m["name"].(string))))
		buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(m["apply_method"].(string))))
		buf.WriteString(fmt.Sprintf("%s-", m["value"].(string)))

		// This hash randomly affects the "order" of the set, which affects in what order parameters
		// are applied, when there are more than 20 (chunked).
		n := stringHashcode(buf.String())

		if old, ok := hashes[n]; ok {
			if old != name {
				panic("Hash collision on " + name)
			}
		}
		hashes[n] = name
		fmt.Println("setting hash name", n, name)
		return n
	}

	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"parameter": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"apply_method": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "immediate",
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Set: resourceParameterHash,
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_db_parameter_group": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider)

	info := tfbridge.ProviderInfo{
		P:           p,
		Name:        "aws",
		Description: "A Pulumi package for creating and managing Amazon Web Services (AWS) cloud resources.",
		Keywords:    []string{"pulumi", "aws"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/phillipedwards/pulumi-aws",
		Version:     "0.0.2",
		Resources: map[string]*tfbridge.ResourceInfo{
			"aws_db_parameter_group": {Tok: "aws:rds/parameterGroup:ParameterGroup"},
		},
	}

	server := tfbridge.NewProvider(ctx,
		nil,      /* hostClient */
		"aws",    /* module */
		"",       /* version */
		p,        /* tf */
		info,     /* info */
		[]byte{}, /* pulumiSchema */
	)

	testCase := `
{
  "method": "/pulumirpc.ResourceProvider/Diff",
  "request": {
    "id": "parametergroup-2b9fa92",
    "urn": "urn:pulumi:dev2::aws-2442::aws:rds/parameterGroup:ParameterGroup::parametergroup",
    "olds": {
      "arn": "arn:aws:rds:us-west-2:616138583583:pg:parametergroup-2b9fa92",
      "description": "Managed by Pulumi",
      "family": "postgres14",
      "id": "parametergroup-2b9fa92",
      "name": "parametergroup-2b9fa92",
      "namePrefix": null,
      "parameters": [
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_analyze",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_buffers",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_format",
          "value": "json"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_min_duration",
          "value": "1000"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_nested_statements",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_timing",
          "value": "0"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_triggers",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.log_verbose",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "auto_explain.sample_rate",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "default_statistics_target",
          "value": "100"
        },
        {
          "applyMethod": "immediate",
          "name": "effective_io_concurrency",
          "value": "200"
        },
        {
          "applyMethod": "immediate",
          "name": "log_autovacuum_min_duration",
          "value": "0"
        },
        {
          "applyMethod": "immediate",
          "name": "log_connections",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "log_disconnections",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "log_lock_waits",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "log_min_duration_statement",
          "value": "1000"
        },
        {
          "applyMethod": "immediate",
          "name": "log_temp_files",
          "value": "1"
        },
        {
          "applyMethod": "immediate",
          "name": "max_parallel_maintenance_workers",
          "value": "4"
        },
        {
          "applyMethod": "immediate",
          "name": "max_parallel_workers_per_gather",
          "value": "4"
        },
        {
          "applyMethod": "immediate",
          "name": "pg_stat_statements.track",
          "value": "ALL"
        },
        {
          "applyMethod": "immediate",
          "name": "random_page_cost",
          "value": "1.1"
        },
        {
          "applyMethod": "immediate",
          "name": "work_mem",
          "value": "65536"
        },
        {
          "applyMethod": "pending-reboot",
          "name": "log_checkpoints",
          "value": "1"
        },
        {
          "applyMethod": "pending-reboot",
          "name": "max_connections",
          "value": "500"
        },
        {
          "applyMethod": "pending-reboot",
          "name": "rds.logical_replication",
          "value": "1"
        },
        {
          "applyMethod": "pending-reboot",
          "name": "shared_preload_libraries",
          "value": "pg_stat_statements,auto_explain"
        },
        {
          "applyMethod": "pending-reboot",
          "name": "track_io_timing",
          "value": "1"
        },
        {
          "applyMethod": "pending-reboot",
          "name": "wal_buffers",
          "value": "2048"
        }
      ],
      "tags": {},
      "tagsAll": {}
    },
    "news": {
      "__defaults": [
        "name"
      ],
      "description": "Managed by Pulumi",
      "family": "postgres14",
      "name": "parametergroup-2b9fa92",
      "parameters": [
        {
          "__defaults": [],
          "applyMethod": "pending-reboot",
          "name": "max_connections",
          "value": "500"
        },
        {
          "__defaults": [],
          "applyMethod": "pending-reboot",
          "name": "wal_buffers",
          "value": "2048"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "default_statistics_target",
          "value": "100"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "random_page_cost",
          "value": "1.1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "effective_io_concurrency",
          "value": "200"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "work_mem",
          "value": "65536"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "max_parallel_workers_per_gather",
          "value": "4"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "max_parallel_maintenance_workers",
          "value": "4"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "pg_stat_statements.track",
          "value": "ALL"
        },
        {
          "__defaults": [],
          "applyMethod": "pending-reboot",
          "name": "shared_preload_libraries",
          "value": "pg_stat_statements,auto_explain"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "track_io_timing",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_min_duration_statement",
          "value": "1000"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_lock_waits",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_temp_files",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_checkpoints",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_connections",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_disconnections",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "log_autovacuum_min_duration",
          "value": "0"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_format",
          "value": "json"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_min_duration",
          "value": "1000"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_analyze",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_buffers",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_timing",
          "value": "0"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_triggers",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_verbose",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.log_nested_statements",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "immediate",
          "name": "auto_explain.sample_rate",
          "value": "1"
        },
        {
          "__defaults": [],
          "applyMethod": "pending-reboot",
          "name": "rds.logical_replication",
          "value": "1"
        }
      ]
    }
  },
  "response": {
    "changes": "DIFF_SOME",
    "diffs": [
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters",
      "parameters"
    ],
    "detailedDiff": {
      "parameters[10].applyMethod": {},
      "parameters[10].name": {},
      "parameters[10].value": {},
      "parameters[14].applyMethod": {},
      "parameters[14].name": {},
      "parameters[14].value": {},
      "parameters[22].applyMethod": {
        "kind": "DELETE"
      },
      "parameters[22].name": {
        "kind": "DELETE"
      },
      "parameters[22].value": {
        "kind": "DELETE"
      },
      "parameters[26].applyMethod": {
        "kind": "DELETE"
      },
      "parameters[26].name": {
        "kind": "DELETE"
      },
      "parameters[26].value": {
        "kind": "DELETE"
      }
    },
    "hasDetailedDiff": true
  }
}`
	testutils.Replay(t, server, testCase)
}
