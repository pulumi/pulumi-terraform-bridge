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

package schemav6

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResoureSchemaExtraction(t *testing.T) {
	x := "x"
	s := schema.Resource{
		Schema: map[string]*schema.Schema{
			x: {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
	es, err := ResourceSchema(&s)
	assert.NoError(t, err)

	found := false
	for _, a := range es.Block.Attributes {
		if a.Name != x {
			continue
		}
		assert.Equal(t, true, a.Optional)
		found = true
	}
	assert.True(t, found)
}

func TestRenderDiagnostics(t *testing.T) {
	assert.Nil(t, renderDiagnostics(nil))

	x := renderDiagnostics([]*tfprotov6.Diagnostic{
		{
			Severity:  tfprotov6.DiagnosticSeverityError,
			Summary:   "summary",
			Detail:    "detail",
			Attribute: tftypes.NewAttributePath().WithAttributeName("foo"),
		},
	})
	assert.Equal(t, "1 unexpected diagnostic(s):\n"+
		`- ERROR at AttributeName("foo"). summary: detail`+"\n", x.Error())
}
