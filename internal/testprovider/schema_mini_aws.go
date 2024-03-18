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

package testprovider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ProviderMiniAws() *schema.Provider {
	resourceBucket := func() *schema.Resource {
		return &schema.Resource{
			Schema:      resourceMiniAwsBucket(),
			Description: "Provides a S3 bucket resource.",
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_s3_bucket": resourceBucket(),
			"aws_s3_bucket_acl": {
				Description: "Provides a S3 bucket ACL resource.",
				Schema:      map[string]*schema.Schema{},
			},
		},
	}
}

func resourceMiniAwsBucket() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"bucket": {
			Type:     schema.TypeString,
			Required: true,
		},
		"acl": {
			Type:     schema.TypeString,
			Optional: true,
			// // NOTE: In the real provider this field is not populated in the terraform schema so we pull it from the
			// // markdown docs. This is the value of the description after being pulled from the markdown docs and after
			// // the doc edits have been applied.
			// Description: "The [canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, and `log-delivery-write`. Defaults to `private`. " +
			// 	"Conflicts with `grant`. The provider will only perform drift detection if a configuration value is provided. Use the resource `aws_s3_bucket_acl` instead.",
			ConflictsWith: []string{"grant"},
			Deprecated:    "Use the aws_s3_bucket_acl resource instead",
		},
	}
}
