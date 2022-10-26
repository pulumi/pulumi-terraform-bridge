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

// Tests expected empty update Diff interaction over the following program:
//
//      r, err := random.NewRandomInteger(ctx, "priority", &random.RandomIntegerArgs{
//          Max: pulumi.Int(50000),
//          Min: pulumi.Int(1),
//      })
func TestDiffRandomEmptyUpdate(t *testing.T) {
	server := tfbridge.NewProviderServer(testprovider.RandomProvider(), []byte{})
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "11187",
            "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
            "olds": {
              "id": "11187",
              "max": 50000,
              "min": 1,
              "result": 11187
            },
            "news": {
              "__defaults": [],
              "max": 50000,
              "min": 1
            }
          },
          "response": {
            "changes": "DIFF_NONE"
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

// The same program but with min field changed, causing a replacement plan.
func TestDiffRandomMinChanged(t *testing.T) {
	server := tfbridge.NewProviderServer(testprovider.RandomProvider(), []byte{})
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "11187",
            "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
            "olds": {
              "id": "11187",
              "max": 50000,
              "min": 1,
              "result": 11187
            },
            "news": {
              "__defaults": [],
              "max": 50000,
              "min": 2
            }
          },
          "response": {
            "replaces": [
              "min"
            ],
            "changes": "DIFF_SOME",
            "diffs": [
              "min"
            ]
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

// Test that preview diff in presence of computed attributes results in an empty diff.
func TestEmptyTestresDiff(t *testing.T) {
	server := tfbridge.NewProviderServer(
		testprovider.SyntheticTestBridgeProvider(),
		testprovider.SyntheticTestBridgeProviderPulumiSchemaBytes(),
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
