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
	"testing"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge"
	testutils "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testprovider"
)

// Test that preview diff in presence of computed attributes results in an empty diff.
func TestEmptyTestresDiff(t *testing.T) {
	schemaBytes := genTestBridgeSchemaBytes(t)
	server := tfbridge.NewProviderServer(
		testprovider.SyntheticTestBridgeProvider(),
		schemaBytes.pulumiSchema,
		schemaBytes.renames,
	)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "0",
            "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:Testres::testres1",
            "olds": {
              "id": "0",
              "requiredInputString": "input1",
              "requiredInputStringCopy": "input1",
              "statedir": "/tmp"
            },
            "news": {
              "requiredInputString": "input1",
              "statedir": "/tmp"
            }
          },
          "response": {
            "changes": "DIFF_NONE"
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

// Test removing an optional input.
func TestOptionRemovalTestresDiff(t *testing.T) {
	schemaBytes := genTestBridgeSchemaBytes(t)
	server := tfbridge.NewProviderServer(
		testprovider.SyntheticTestBridgeProvider(),
		schemaBytes.pulumiSchema,
		schemaBytes.renames,
	)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "0",
            "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:Testres::testres1",
            "olds": {
              "id": "0",
              "requiredInputString": "input1",
              "optionalInputString": "input2",
              "requiredInputStringCopy": "input3",
              "statedir": "/tmp"
            },
            "news": {
              "requiredInputString": "input1",
              "statedir": "/tmp"
            }
          },
          "response": {
            "changes": "DIFF_SOME",
            "diffs": [
               "optionalInputString"
            ]
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

// Make sure optionalInputBoolCopy does not cause non-empty diff when not actually changing.
func TestEmptyTestresDiffWithOptionalComputed(t *testing.T) {
	schemaBytes := genTestBridgeSchemaBytes(t)
	server := tfbridge.NewProviderServer(
		testprovider.SyntheticTestBridgeProvider(),
		schemaBytes.pulumiSchema,
		schemaBytes.renames,
	)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "0",
            "urn": "urn:pulumi:dev12::basicprogram::testbridge:index/testres:Testres::testres5",
            "olds": {
              "id": "0",
              "optionalInputBool": true,
              "optionalInputBoolCopy": true,
              "requiredInputString": "x",
              "requiredInputStringCopy": "x",
              "statedir": "state"
            },
            "news": {
              "optionalInputBool": true,
              "requiredInputString": "x",
              "statedir": "state"
            }
          },
          "response": {
            "changes": "DIFF_NONE"
          }
        }`
	testutils.Replay(t, server, testCase)
}
