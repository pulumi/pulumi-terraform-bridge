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

// Test the following program:
//
//      r, err := random.NewRandomInteger(ctx, "priority", &random.RandomIntegerArgs{
//          Max: pulumi.Int(50000),
//          Min: pulumi.Int(1), // change to 2
//     	    Seed: pulumi.String("my-random-seed"),
//      })
//
// The plan indicates replacement which eventually calls Create. Test
// that Create behaves as expected.
func TestCreateRandomMinChanged(t *testing.T) {

	t.Run("preview", func(t *testing.T) {
		server := tfbridge.NewProviderServer(testprovider.RandomProvider(), []byte{})
		testCase := `
                {
                  "method": "/pulumirpc.ResourceProvider/Create",
                  "request": {
                    "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
                    "properties": {
                      "__defaults": [],
                      "max": 50000,
                      "min": 2,
                      "seed": "my-random-seed"
                    },
                    "preview": true
                  },
                  "response": {
                    "properties": {
                      "max": 50000,
                      "min": 2,
                      "seed": "my-random-seed"
                    }
                  }
                }
                `
		testutils.Replay(t, server, testCase)
	})

	t.Run("update", func(t *testing.T) {
		server := tfbridge.NewProviderServer(testprovider.RandomProvider(), []byte{})
		testCase := `
                {
                  "method": "/pulumirpc.ResourceProvider/Create",
                  "request": {
                    "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
                    "properties": {
                      "__defaults": [],
                      "seed": "my-random-seed",
                      "max": 50000,
                      "min": 2
                    }
                  },
                  "response": {
                    "id": "7395",
                    "properties": {
                      "id": "7395",
                      "max": 50000,
                      "min": 2,
                      "result": 7395,
                      "seed": "my-random-seed"
                    }
                  }
                }
                `
		testutils.Replay(t, server, testCase)
	})
}
