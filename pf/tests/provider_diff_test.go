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

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
)

// Test that preview diff in presence of computed attributes results in an empty diff.
func TestEmptyTestresDiff(t *testing.T) {
	server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
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
	server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
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
	server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
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

func TestDiffWithSecrets(t *testing.T) {
	server := newProviderServer(t, testprovider.RandomProvider())

	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "none",
            "urn": "urn:pulumi:p-it-antons-mac-ts-c25899e1::simple-random::random:index/randomPassword:RandomPassword::password",
            "olds": {
              "bcryptHash": {
                "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                "value": "$2a$10$kO5OLyiYS.C1IA/Es/wJPugt9F9GM4jKMLmZW5gjUwP5RB4OJ6WTK"
              },
              "id": "none",
              "length": 32,
              "lower": true,
              "minLower": 0,
              "minNumeric": 0,
              "minSpecial": 0,
              "minUpper": 0,
              "number": true,
              "numeric": true,
              "result": {
                "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                "value": ":7WJQW2-7D%*fp:s$L!8ABF}_$A{L8>j"
              },
              "special": true,
              "upper": true
            },
            "news": {
              "length": 32
            }
          },
          "response": {
            "changes": "DIFF_NONE"
          }
        }`
	testutils.Replay(t, server, testCase)
}

// See https://github.com/pulumi/pulumi-random/issues/258
func TestDiffVersionUpgrade(t *testing.T) {
	server := newProviderServer(t, testprovider.RandomProvider())
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Diff",
          "request": {
            "id": "none",
            "urn": "urn:pulumi:dev::repro-pulumi-random-258::random:index/randomPassword:RandomPassword::access-token-",
            "olds": {
              "__meta": "{\"schema_version\":\"1\"}",
              "bcryptHash": {
                "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                "value": "$2a$10$S97m.MSYGqJwssuBpH9.ge/2.b5bJgQG4EqqNdj8SAaRYowINbaW6"
              },
              "id": "none",
              "length": 16,
              "lower": true,
              "minLower": 0,
              "minNumeric": 0,
              "minSpecial": 0,
              "minUpper": 0,
              "number": true,
              "overrideSpecial": "_%@:",
              "result": {
                "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                "value": "6c83gZo_72VOYy6l"
              },
              "special": true,
              "upper": true
            },
            "news": {
              "length": 16,
              "overrideSpecial": "_%@:",
              "special": true
            }
          },
          "response": {
            "changes": "DIFF_NONE"
          }
        }`
	testutils.Replay(t, server, testCase)
}
