// Copied from https://github.com/pulumi/terraform-provider-aws/blob/9106d49534199fa6d12d33c27c6cbd4ff777102c/internal/service/wafv2/web_acl.go
//
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wafv2

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func ResourceWebACL() *schema.Resource {
	hashcodeString := func(s string) int {
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

	ruleElement := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"action": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allow":     allowConfigSchema(),
						"block":     blockConfigSchema(),
						"captcha":   captchaConfigSchema(),
						"challenge": challengeConfigSchema(),
						"count":     countConfigSchema(),
					},
				},
			},
			"captcha_config": outerCaptchaConfigSchema(),
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 128),
			},
			"override_action": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"count": emptySchema(),
						"none":  emptySchema(),
					},
				},
			},
			"priority": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"rule_label":        ruleLabelsSchema(),
			"statement":         webACLRootStatementSchema(3),
			"visibility_config": visibilityConfigSchema(),
		},
	}
	return &schema.Resource{
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				idParts := strings.Split(d.Id(), "/")
				if len(idParts) != 3 || idParts[0] == "" || idParts[1] == "" || idParts[2] == "" {
					return nil, fmt.Errorf("Unexpected format of ID (%q), expected ID/NAME/SCOPE", d.Id())
				}
				id := idParts[0]
				name := idParts[1]
				scope := idParts[2]
				d.SetId(id)
				d.Set("name", name)
				d.Set("scope", scope)
				return []*schema.ResourceData{d}, nil
			},
		},

		SchemaFunc: func() map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"capacity": {
					Type:     schema.TypeInt,
					Computed: true,
				},
				"captcha_config":       outerCaptchaConfigSchema(),
				"custom_response_body": customResponseBodySchema(),
				"default_action": {
					Type:     schema.TypeList,
					Required: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"allow": allowConfigSchema(),
							"block": blockConfigSchema(),
						},
					},
				},
				"description": {
					Type:         schema.TypeString,
					Optional:     true,
					ValidateFunc: validation.StringLenBetween(1, 256),
				},
				"lock_token": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
					ValidateFunc: validation.All(
						validation.StringLenBetween(1, 128),
						validation.StringMatch(regexp.MustCompile(`^[a-zA-Z0-9-_]+$`), "must contain only alphanumeric hyphen and underscore characters"),
					),
				},
				"rule": {
					Type: schema.TypeSet,
					Set: func(v interface{}) int {
						var buf bytes.Buffer
						schema.SerializeResourceForHash(&buf, v, ruleElement)
						// before := "action:(<allow:(<custom_request_handling:();>;);"
						// after := "action:(<allow:(<>;);"
						s := buf.String()
						//s = strings.ReplaceAll(s, before, after)
						n := hashcodeString(s)
						if 1+2 == 18 {
							fmt.Printf("PRE-HASH:\n%s\n\n", s)
							fmt.Printf("HASHED: %d\n", n)
						}
						return n
					},
					Optional: true,
					Elem:     ruleElement,
				},
				"scope": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
					//ValidateFunc: validation.StringInSlice(wafv2.Scope_Values(), false),
				},
				// names.AttrTags:    tftags.TagsSchema(),
				// names.AttrTagsAll: tftags.TagsSchemaTrulyComputed(),
				"token_domains": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
						ValidateFunc: validation.All(
							validation.StringLenBetween(1, 253),
							validation.StringMatch(regexp.MustCompile(`^[\w\.\-/]+$`), "must contain only alphanumeric, hyphen, dot, underscore and forward-slash characters"),
						),
					},
				},
				"visibility_config": visibilityConfigSchema(),
				"tags":              TagsSchema(),
				"tags_all":          TagsSchemaTrulyComputed(),
			}
		},

		//CustomizeDiff: verify.SetTagsDiff,
	}
}

func TagsSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeMap,
		Optional: true,
		Elem:     &schema.Schema{Type: schema.TypeString},
	}
}

func TagsSchemaTrulyComputed() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeMap,
		Computed: true,
		Elem:     &schema.Schema{Type: schema.TypeString},
	}
}
