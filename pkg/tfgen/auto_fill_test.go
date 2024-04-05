// Copyright 2016-2024, Pulumi Corporation.
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

package tfgen

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestAutoFill(t *testing.T) {
	example := `
resource "aws_route53_record" "example" {
      for_each = {
        for dvo in aws_acm_certificate.example.domain_validation_options : dvo.domain_name => {
          name   = dvo.resource_record_name
          record = dvo.resource_record_value
          type   = dvo.resource_record_type
        }
      }

      allow_overwrite = true
      name            = each.value.name
      records         = [each.value.record]
      ttl             = 60
      type            = each.value.type
      zone_id         = aws_route53_zone.example.zone_id
}`

	injectAcmCert := `
resource "aws_acm_certificate" "example" {
  domain_name       = "example.com"
  validation_method = "DNS"
}`

	injectRoute53Zone := `
resource "aws_route53_zone" "example" {
  name = "example.com"
}`

	fs := afero.NewMemMapFs()

	err := afero.WriteFile(fs, "aws_acm_certificate.tf", []byte(injectAcmCert), 0600)
	require.NoError(t, err)

	err = afero.WriteFile(fs, "aws_route53_zone.tf", []byte(injectRoute53Zone), 0600)
	require.NoError(t, err)

	taf := NewFolderBasedAutoFiller(fs)

	actual, err := AutoFill(taf, example)
	require.NoError(t, err)

	t.Logf("ACTUAL: %s", actual)

	autogold.ExpectFile(t, actual)
}
