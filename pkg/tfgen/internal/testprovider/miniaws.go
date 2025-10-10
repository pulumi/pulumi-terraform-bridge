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
	"bytes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	testproviderdata "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func ProviderMiniAws() info.Provider {
	return info.Provider{
		P:           shimv2.NewProvider(testproviderdata.ProviderMiniAws()),
		Name:        "aws",
		Description: "A Pulumi package to safely use aws in Pulumi programs.",
		Keywords:    []string{"pulumi", "aws"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/pulumi/pulumi-aws",
		DocRules: &info.DocRule{EditRules: func(defaults []info.DocsEdit) []info.DocsEdit {
			return []info.DocsEdit{
				{
					Path: "*",
					Edit: func(_ string, content []byte) ([]byte, error) {
						content = bytes.ReplaceAll(
							content,
							// This replacement is done in the aws provider
							// here. This replacement is necessary because the
							// bridge will drop any docs that contain the word
							// 'Terraform'
							// https://github.com/pulumi/pulumi-aws/blob/df5d52299c72b936df9c9289d83d10225dc1dce7/provider/replacements.json#L1688
							//nolint:lll
							[]byte(" Terraform will only perform drift detection if a configuration value is provided."),
							[]byte(" The provider will only perform drift detection if a configuration value is provided."),
						)
						return content, nil
					},
				},
			}
		}},
		UpstreamRepoPath: "./test_data",
		Resources: map[string]*info.Resource{
			"aws_s3_bucket_acl": {
				Tok: tokens.Type(tokens.ModuleMember("aws:s3/bucketAclV2:BucketAclV2")),
			},
			"aws_s3_bucket": {
				Tok: tokens.Type(tokens.ModuleMember("aws:s3/bucketV2:BucketV2")),
				Aliases: []info.Alias{
					{
						Type: ref("aws:s3/bucket:Bucket"),
					},
				},
			},
		},
	}
}

func ref[T any](value T) *T { return &value }
