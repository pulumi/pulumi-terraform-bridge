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

// This tests the viability of the workaround for TLS provider major version upgrade.
//
// Replace plans are not acceptable. The ignoreChanges option should work to avoid replace plans on setSubjectKeyId,
// setAuthorityKeyId, which otherwise cannot be avoided.
//
// See https://github.com/pulumi/pulumi-tls/issues/173 for the details.
func TestTlsResourceUpgrade(t *testing.T) {
	server := newProviderServer(t, testprovider.TlsProvider())
	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Diff",
	  "request": {
            "ignoreChanges": ["setAuthorityKeyId", "setSubjectKeyId"],
	    "id": "148560014228468229219907850397611105717",
	    "urn": "urn:pulumi:repro::provider-upgrade::tls:index/selfSignedCert:SelfSignedCert::test-cert",
	    "olds": {
	      "certPem": "-----BEGIN CERTIFICATE-----\nMIIC2jCCAcKgAwIBAgIQb8OeNNzW217eEwlpNnqdtTANBgkqhkiG9w0BAQsFADAP\nMQ0wCwYDVQQDEwR0ZXN0MB4XDTIzMDIxNTE0NTMwM1oXDTI1MDIxNTAyNTMwM1ow\nDzENMAsGA1UEAxMEdGVzdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB\nAKdCyYqgcLZ+8vINlkMlZxQOm8cGYrl58Xk2/nuLg8uuCpNSa4BxL0gKmMBiwYeR\nPzPbzGlhT2okresY4aJJtIinKmQiygJhzANjpBB5tZfGGh1V6Q20bJIh3pwG+o1h\ndLkMi6Q2W8enpV7QODVTxbrykv7LqwDk0Sz/r0sEHuYZDflwjXuzoCkwk/KVV7Wq\nMEvl0bfYwU+oIERpYDnaRt1ZNwtlP1qJMm0GGAakHII550bmMjhKbi28DDOGPve6\nTFGBt84du8wzqtagXqRjRiOKQ79I4wjUSWAW1A08GqgECWC8kck9Txb9COdroqqZ\nDtfmlnWeUqXtBMLLVXejy5sCAwEAAaMyMDAwDwYDVR0TAQH/BAUwAwEB/zAdBgNV\nHQ4EFgQU5lz5FGBgZvtKSTVlVqXA0EOzfJMwDQYJKoZIhvcNAQELBQADggEBAAik\nuBStQKb96DJeUhv5yTGU7ZbHuzl5V6++HWPGw4mjPvWkVx9TLgASEg0ZwgGxSzzj\nmsLDf1//4pZhZRXjUMLJ+ws4wFu8vDlDTEydbrK5ld1GQPuvCg/IIvw4KXIC79cu\nJOqd0z0/ohmp7FHBFQK9CX+it9C20Koj3w/rKfUlA6Ficdfht7F5p5viXUs/xo7g\n5Mcw2OTwirQtVbXQKZ7vyr/HoawZMG3S7lkgktAqlRXFQ8LKWE0UJL2TJmKDm2Mv\nIBDtZuRJ0shdEnWa76BgSyg+dzd55Ow6BfJCAWvoxa4EGp3ZJUH5B704Y7U3Yus8\nY1lMvs7b2B6W7IRN9qE=\n-----END CERTIFICATE-----\n",
	      "earlyRenewalHours": 0,
	      "id": "148560014228468229219907850397611105717",
	      "isCaCertificate": true,
	      "keyAlgorithm": "RSA",
	      "privateKeyPem": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "dfc12ec4e5378e9467d46f5216acf540f1a68e89"
	      },
	      "readyForRenewal": false,
	      "subject": {
		"commonName": "test",
		"country": "",
		"locality": "",
		"organization": "",
		"organizationalUnit": "",
		"postalCode": "",
		"province": "",
		"serialNumber": "",
		"streetAddresses": []
	      },
	      "validityEndTime": "2025-02-14T21:53:03.468703-05:00",
	      "validityPeriodHours": 17532,
	      "validityStartTime": "2023-02-15T09:53:03.468703-05:00"
	    },
	    "news": {
	      "allowedUses": null,
	      "isCaCertificate": true,
	      "privateKeyPem": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEAp0LJiqBwtn7y8g2WQyVnFA6bxwZiuXnxeTb+e4uDy64Kk1Jr\ngHEvSAqYwGLBh5E/M9vMaWFPaiSt6xjhokm0iKcqZCLKAmHMA2OkEHm1l8YaHVXp\nDbRskiHenAb6jWF0uQyLpDZbx6elXtA4NVPFuvKS/surAOTRLP+vSwQe5hkN+XCN\ne7OgKTCT8pVXtaowS+XRt9jBT6ggRGlgOdpG3Vk3C2U/WokybQYYBqQcgjnnRuYy\nOEpuLbwMM4Y+97pMUYG3zh27zDOq1qBepGNGI4pDv0jjCNRJYBbUDTwaqAQJYLyR\nyT1PFv0I52uiqpkO1+aWdZ5Spe0EwstVd6PLmwIDAQABAoIBAQCbf3vfZUlkYKF8\nZyVLR3qNKweoAEfIJ5ZXGsl8Ejh1I1ixne5TeuZ6E1/ve+BwKJiZnb5sOguaon8O\nEhOyzNMKOF8wuScVD9abUAc3Se+JKqMcosIH+7T0JojOha5pwjDB2Of5wo+RDkqv\n2uRmr3skUmBWgQJ50kCllQ9irnILd73OXgYlEbim0lR2V1T6Ksgb52MvTPjk/bzS\nv4Cp4YQZC7z7qqYWZ2/Jpt7fve3212Jy986KHlj2KluaRjtdq2AxwYogwd5CkS4E\nm1L8Mj1z5zknE5bocl9OdFMsvFK/Ok1hL2VqpxfondapdHjkI1Bbfl3dCU+xYg0y\ntabdk3ABAoGBAMDLjp+udI/KVyyfAbPlKA/PXVgWlJ82mmsw6mLrw3UbWsbd59Gl\nwKwJUJQrwyA4oJZtg95EoQtapInqEkJMgI9ZRnDfFp/RJbDaIQDe1U2mCJkILfYU\n3PPgE/UfuZP3B3XTN5tQJ0bNFfVelD4gvbWWFDXDJ7XAXsOYX2e/T0TnAoGBAN4Y\nQMRggeWlQcJzH7F0iv3E8Eb64QGeeJcofHzcvEXy3lh7k1ZOiqZzv5Aya/P/oHhV\nxyl00U5NKXN6QIjjQ7JcH5jOcIZzDpQ/ao+wjs3XtJHqablW4ph0XZ7J8u6CJFJc\nl31a6nVmLlb/66911tNYnff6GNhY+heqrbQX0PktAoGAKL8czpzVX8p48CJO/tFQ\nzT6bUNG86YVlz3/QGcYQUkDMx7kAlKt+dB2n3Rj+rWGqdwCAXUqN6tNmcQt6fm6i\nwSkyHQrZQj+2wpDnZsKxvC56JLW42QiBxj02mpjw5NfRyNIyL24aTvlrSaeKlzLe\nRXGJpe8wBla48IfUqh2hyEMCgYEApkN1yQ2OcQLENfPFWC2tF8llL14FMBcYo+CV\nQUxmTd9BgPASHtxxg6bHVAXLN0C5OxzMGkbvojS1wVNWGKQ6O74nkVeKebyMv4Ky\nHZvJbGP9M/dO6ocW35bNt1/r043t7xKN/jQfrX+vVUYFhLcs+c8vg0LhcqU5pJoL\nq/TgZokCgYEAozW2b5W8KK2KNoY+/LmA0Y5OWGP2MPF0CfaI8faXQ//VoKfnfzab\nMeHICphMLW4aWJkN+9iLOoRx+v9Mb1cvvr8nBR6s/NpygOl69wC0q1uaXaxDilYM\n5u71BeNcpMJdfZlsBH6Ev81TG26EEw6tVKnUJOC0oHF5kHIokYvty24=\n-----END RSA PRIVATE KEY-----\n"
	      },
	      "setAuthorityKeyId": false,
	      "subject": {
		"commonName": "test",
		"country": "",
		"locality": "",
		"organization": "",
		"organizationalUnit": "",
		"postalCode": "",
		"province": "",
		"serialNumber": "",
		"streetAddresses": []
	      },
	      "validityPeriodHours": 17532
	    }
	  },
	  "response": {
	    "changes": "DIFF_SOME",
            "diffs": [
              "privateKeyPem"
            ]
	  }
	}
        `
	testutils.Replay(t, server, testCase)
}
