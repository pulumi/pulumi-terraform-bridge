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

package tfbridgetests

import (
	"testing"

	testutils "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testing"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
)

func TestReadFromRefresh(t *testing.T) {
	// This test case was obtained by running `pulumi refresh` on a simple stack with one RandomPassword.
	//
	// Specifically testing for:
	//
	// - __meta writing out the schema version
	// - inputs being populated
	server := newProviderServer(t, testprovider.RandomProvider())
	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "none",
	    "urn": "urn:pulumi:dev::repro-pulumi-random-258::random:index/randomPassword:RandomPassword::access-token-",
	    "properties": {
	      "__meta": "{\"schema_version\":\"1\"}",
	      "bcryptHash": "$2a$10$HHwx0gQztkpPIc7WkE4Wt.v7ibWT9Ug24/F5XLa6xNm/gOuyS5WRa",
	      "id": "none",
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "result": "Ps7XGKxa",
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
	      "__defaults": [
		"lower",
		"minLower",
		"minNumeric",
		"minSpecial",
		"minUpper",
		"number",
		"upper"
	      ],
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "special": true,
	      "upper": true
	    }
	  },
	  "response": {
	    "id": "none",
	    "properties": {
	      "__meta": "{\"schema_version\":\"1\"}",
	      "bcryptHash": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "$2a$10$HHwx0gQztkpPIc7WkE4Wt.v7ibWT9Ug24/F5XLa6xNm/gOuyS5WRa"
	      },
	      "id": "none",
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "result": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "Ps7XGKxa"
	      },
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
	      "__defaults": [
		"lower",
		"minLower",
		"minNumeric",
		"minSpecial",
		"minUpper",
		"number",
		"upper"
	      ],
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "special": true,
	      "upper": true
	    }
	  }
	}`
	testutils.Replay(t, server, testCase)
}
