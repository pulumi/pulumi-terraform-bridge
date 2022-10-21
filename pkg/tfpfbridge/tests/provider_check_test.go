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

// Tests expected empty update Check interaction over the following program:
//
//      r, err := random.NewRandomInteger(ctx, "priority", &random.RandomIntegerArgs{
//          Max: pulumi.Int(50000),
//          Min: pulumi.Int(1),
//      })
func TestCheckRandomEmptyUpdate(t *testing.T) {
	server := tfbridge.NewProviderServer(testprovider.RandomProvider())
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Check",
          "request": {
            "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
            "olds": {
              "__defaults": [],
              "max": 50000,
              "min": 1
            },
            "news": {
              "max": 50000,
              "min": 1
            },
            "randomSeed": "2MqG3voL18gmPMCcB/N+XcwD0P5vSwvYJE5wkm0Hj0k="
          },
          "response": {
            "inputs": {
              "__defaults": [],
              "max": 50000,
              "min": 1
            }
          },
          "metadata": {
            "bin": "/Users/anton/.pulumi/plugins/resource-random-v4.8.2/pulumi-resource-random",
            "mode": "client",
            "port": 60216,
            "prefix": "random (resource)"
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

// The same program but with min field changed, causing a replacement plan.
func TestCheckRandomMinChanged(t *testing.T) {
	server := tfbridge.NewProviderServer(testprovider.RandomProvider())
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Check",
          "request": {
            "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
            "olds": {
              "__defaults": [],
              "max": 50000,
              "min": 1
            },
            "news": {
              "max": 50000,
              "min": 2
            },
            "randomSeed": "Wa6G/sfG/U97HvFbEVt2NnQoo6S4Ft+dl5UYop8MWRc="
          },
          "response": {
            "inputs": {
              "__defaults": [],
              "max": 50000,
              "min": 2
            }
          }
        }
        `
	testutils.Replay(t, server, testCase)
}
