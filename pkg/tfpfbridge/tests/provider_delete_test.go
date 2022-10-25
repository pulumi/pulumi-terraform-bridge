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
// The plan indicates replacement which eventually calls Delete. Test
// that Delete behaves as expected.
func TestDeleteRandomMinChanged(t *testing.T) {
	server := tfbridge.NewProviderServer(testprovider.RandomProvider(), []byte{})
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Delete",
          "request": {
            "id": "49376",
            "urn": "urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority",
            "properties": {
              "id": "49376",
              "max": 50000,
              "min": 1,
              "result": 49376,
              "seed": "my-random-seed"
            }
          },
          "response": {}
        }
        `
	testutils.Replay(t, server, testCase)
}
