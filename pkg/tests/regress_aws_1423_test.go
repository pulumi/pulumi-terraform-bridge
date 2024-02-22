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
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/providertest/replay"
	webaclschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/webaclschema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegressAws1423(t *testing.T) {
	t.Skip("Refresh is dirty on this resource see https://github.com/pulumi/pulumi-aws/issues/3361.")
	ctx := context.Background()

	resource := webaclschema.ResourceWebACL()

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_wafv2_web_acl": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider, shimv2.WithDiffStrategy(shimv2.PlanState),
		shimv2.WithPlanResourceChange(func(s string) bool {
			switch s {
			case "aws_wafv2_web_acl":
				return true
			default:
				return false
			}
		}),
	)

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
			"aws_wafv2_web_acl": {Tok: "aws:wafv2/webAcl:WebAcl"},
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

	testCase1 := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::bridge-887::aws:wafv2/webAcl:WebAcl::aclw",
	    "properties": {
	      "__defaults": [
		"name"
	      ],
	      "name": "aclw-a956aa7",
	      "rules": [
		{
		  "__defaults": [],
		  "name": "rule-1",
		  "action": null,
		  "captchaConfig": null,
		  "priority": 0,
		  "overrideAction": {
		    "__defaults": [],
		    "count": {
		      "__defaults": []
		    }
		  }
		}
	      ]
	    },
	    "preview": true
	  },
	  "response": {
	    "properties": {
              "tags": null,
              "tagsAll": "*",
	      "arn": "*",
	      "capacity": "*",
	      "captchaConfig": "*",
	      "customResponseBodies": [],
	      "defaultAction": null,
	      "description": null,
	      "lockToken": "*",
	      "id": "*",
	      "name": "aclw-a956aa7",
	      "scope": "",
	      "tokenDomains": null,
	      "visibilityConfig": null,
	      "rules": [
		{
		  "action": null,
		  "captchaConfig": null,
		  "ruleLabels": [],
		  "statement": null,
		  "visibilityConfig": null,
		  "name": "rule-1",
		  "overrideAction": {
		    "count": {},
		    "none": null
		  },
		  "priority": 0
		}
	      ]
	    }
	  }
	}`
	t.Run("testCase1", func(t *testing.T) {
		testutils.Replay(t, server, testCase1)
	})

	testCase2CreatePreview := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::aws-2264::aws:wafv2/webAcl:WebAcl::my-web-acl",
	    "properties": {
	      "__defaults": [],
	      "defaultAction": {
		"__defaults": [],
		"block": {
		  "__defaults": []
		}
	      },
	      "name": "my-web-acl",
	      "rules": [
		{
		  "__defaults": [],
		  "action": {
		    "__defaults": [],
		    "allow": {
		      "__defaults": []
		    }
		  },
		  "name": "US-access-only",
		  "priority": 0,
		  "statement": {
		    "__defaults": [],
		    "geoMatchStatement": {
		      "__defaults": [],
		      "countryCodes": [
			"US"
		      ]
		    }
		  },
		  "visibilityConfig": {
		    "__defaults": [],
		    "cloudwatchMetricsEnabled": true,
		    "metricName": "US-access-only",
		    "sampledRequestsEnabled": true
		  }
		}
	      ],
	      "scope": "REGIONAL",
	      "visibilityConfig": {
		"__defaults": [],
		"cloudwatchMetricsEnabled": true,
		"metricName": "my-web-acl",
		"sampledRequestsEnabled": true
	      }
	    },
	    "preview": true
	  },
	  "response": {
	    "properties": {
              "tokenDomains": null,
              "description": null, "tags": null, "tagsAll": "*",
	      "arn": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	      "capacity": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	      "captchaConfig": null,
	      "customResponseBodies": [],
	      "defaultAction": {
		"allow": null,
		"block": {"customResponse": null}
	      },
	      "id": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	      "lockToken": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	      "name": "my-web-acl",
	      "rules": [
		{
		  "action": {
		    "allow": {
		      "customRequestHandling": null
		    },
		    "block": null,
		    "captcha": null,
		    "challenge": null,
		    "count": null
		  },
		  "captchaConfig": null,
		  "name": "US-access-only",
		  "overrideAction": null,
		  "priority": 0,
		  "ruleLabels": [],
		  "statement": {
		    "andStatement": null,
		    "byteMatchStatement": null,
		    "geoMatchStatement": {
		      "countryCodes": [
			"US"
		      ],
		      "forwardedIpConfig": null
		    },
		    "ipSetReferenceStatement": null,
		    "labelMatchStatement": null,
		    "managedRuleGroupStatement": null,
		    "notStatement": null,
		    "orStatement": null,
		    "rateBasedStatement": null,
		    "regexMatchStatement": null,
		    "regexPatternSetReferenceStatement": null,
		    "ruleGroupReferenceStatement": null,
		    "sizeConstraintStatement": null,
		    "sqliMatchStatement": null,
		    "xssMatchStatement": null
		  },
		  "visibilityConfig": {
		    "cloudwatchMetricsEnabled": true,
		    "metricName": "US-access-only",
		    "sampledRequestsEnabled": true
		  }
		}
	      ],
	      "scope": "REGIONAL",
	      "visibilityConfig": {
		"cloudwatchMetricsEnabled": true,
		"metricName": "my-web-acl",
		"sampledRequestsEnabled": true
	      }
	    }
	  }
	}
        `

	testCase2 := `
	{
	  "method": "/pulumirpc.ResourceProvider/Diff",
	  "request": {
	    "id": "6ff5298f-7bcd-4e73-ba75-8784c2c77bcb",
	    "urn": "urn:pulumi:dev::aws-2264::aws:wafv2/webAcl:WebAcl::my-web-acl-2",
	    "olds": {
	      "arn": "arn:aws:wafv2:us-east-1:616138583583:regional/webacl/my-web-acl-2/6ff5298f-7bcd-4e73-ba75-8784c2c77bcb",
	      "associationConfig": null,
	      "capacity": 1,
	      "captchaConfig": null,
	      "customResponseBodies": [],
	      "defaultAction": {
		"allow": null,
		"block": {
		  "customResponse": null
		}
	      },
	      "description": "",
	      "id": "6ff5298f-7bcd-4e73-ba75-8784c2c77bcb",
	      "lockToken": "a58a7783-ac87-4ba9-8801-324ae5c63a07",
	      "name": "my-web-acl-2",
	      "rules": [
		{
		  "action": {
		    "allow": {
		      "customRequestHandling": null
		    },
		    "block": null,
		    "captcha": null,
		    "challenge": null,
		    "count": null
		  },
		  "captchaConfig": null,
		  "name": "US-access-only",
		  "overrideAction": null,
		  "priority": 0,
		  "ruleLabels": [],
		  "statement": {
		    "andStatement": null,
		    "byteMatchStatement": null,
		    "geoMatchStatement": {
		      "countryCodes": [
			"US"
		      ],
		      "forwardedIpConfig": null
		    },
		    "ipSetReferenceStatement": null,
		    "labelMatchStatement": null,
		    "managedRuleGroupStatement": null,
		    "notStatement": null,
		    "orStatement": null,
		    "rateBasedStatement": null,
		    "regexMatchStatement": null,
		    "regexPatternSetReferenceStatement": null,
		    "ruleGroupReferenceStatement": null,
		    "sizeConstraintStatement": null,
		    "sqliMatchStatement": null,
		    "xssMatchStatement": null
		  },
		  "visibilityConfig": {
		    "cloudwatchMetricsEnabled": true,
		    "metricName": "US-access-only",
		    "sampledRequestsEnabled": true
		  }
		}
	      ],
	      "scope": "REGIONAL",
	      "tags": {},
	      "tagsAll": {},
	      "tokenDomains": [],
	      "visibilityConfig": {
		"cloudwatchMetricsEnabled": true,
		"metricName": "my-web-acl-2",
		"sampledRequestsEnabled": true
	      }
	    },
	    "news": {
	      "__defaults": [],
	      "defaultAction": {
		"__defaults": [],
		"block": {
		  "__defaults": []
		}
	      },
	      "name": "my-web-acl-2",
	      "rules": [
		{
		  "__defaults": [],
		  "action": {
		    "__defaults": [],
		    "allow": {
		      "__defaults": []
		    }
		  },
		  "name": "US-access-only",
		  "priority": 0,
		  "statement": {
		    "__defaults": [],
		    "geoMatchStatement": {
		      "__defaults": [],
		      "countryCodes": [
			"US"
		      ]
		    }
		  },
		  "visibilityConfig": {
		    "__defaults": [],
		    "cloudwatchMetricsEnabled": true,
		    "metricName": "US-access-only",
		    "sampledRequestsEnabled": true
		  }
		}
	      ],
	      "scope": "REGIONAL",
	      "visibilityConfig": {
		"__defaults": [],
		"cloudwatchMetricsEnabled": true,
		"metricName": "my-web-acl-2",
		"sampledRequestsEnabled": true
	      }
	    },
	    "oldInputs": {
	      "__defaults": [],
	      "defaultAction": {
		"__defaults": [],
		"block": {
		  "__defaults": []
		}
	      },
	      "name": "my-web-acl-2",
	      "rules": [
		{
		  "__defaults": [],
		  "action": {
		    "__defaults": [],
		    "allow": {
		      "__defaults": []
		    }
		  },
		  "name": "US-access-only",
		  "priority": 0,
		  "statement": {
		    "__defaults": [],
		    "geoMatchStatement": {
		      "__defaults": [],
		      "countryCodes": [
			"US"
		      ]
		    }
		  },
		  "visibilityConfig": {
		    "__defaults": [],
		    "cloudwatchMetricsEnabled": true,
		    "metricName": "US-access-only",
		    "sampledRequestsEnabled": true
		  }
		}
	      ],
	      "scope": "REGIONAL",
	      "visibilityConfig": {
		"__defaults": [],
		"cloudwatchMetricsEnabled": true,
		"metricName": "my-web-acl-2",
		"sampledRequestsEnabled": true
	      }
	    }
	  },
	  "response": {
	    "stables": "*",
	    "changes": "DIFF_NONE",
	    "hasDetailedDiff": true
	  }
	}`

	t.Run("testCase2/createPreview", func(t *testing.T) {
		// This is wrong; this test case is from a preview after an up wihtout edits, it should not detect
		// diffs.
		testutils.Replay(t, server, testCase2CreatePreview)
	})
	t.Run("testCase2/diff", func(t *testing.T) {
		// This is wrong; this test case is from a preview after an up wihtout edits, it should not detect
		// diffs.
		testutils.Replay(t, server, testCase2)
	})
}
