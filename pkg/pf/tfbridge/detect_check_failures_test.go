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

package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestDetectCheckFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := &provider{}
	urn := resource.NewURN("stack", "project", "", "typ", "name")

	t.Run("ignores warnings", func(t *testing.T) {
		cf := p.detectCheckFailure(ctx, urn, false, nil, nil, &tfprotov6.Diagnostic{
			Severity:  tfprotov6.DiagnosticSeverityWarning,
			Attribute: tftypes.NewAttributePath().WithAttributeName("a"),
		})
		assert.Nil(t, cf)
	})

	t.Run("propagates errors", func(t *testing.T) {
		cf := p.detectCheckFailure(ctx, urn, false, nil, nil, &tfprotov6.Diagnostic{
			Severity:  tfprotov6.DiagnosticSeverityError,
			Attribute: tftypes.NewAttributePath().WithAttributeName("a"),
			Summary:   "Bad things happening",
		})
		assert.NotNil(t, cf)
		assert.Equal(t, "Bad things happening. Examine values at 'name.a'.", cf.Reason)
	})

	t.Run("ignores errors not specific to an attribute", func(t *testing.T) {
		cf := p.detectCheckFailure(ctx, urn, false, nil, nil, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Bad things happening",
		})
		assert.Nil(t, cf)
	})
}
