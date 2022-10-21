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

package tfbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
)

func TestTerraformResourceName(t *testing.T) {
	urn := resource.URN("urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority")
	p := &Provider{info: info.ProviderInfo{
		Resources: map[string]*info.ResourceInfo{
			"random_integer": {Tok: "random:index/randomInteger:RandomInteger"},
		},
	}}
	name, err := p.terraformResourceName(urn.Type())
	assert.NoError(t, err)
	assert.Equal(t, name, "random_integer")
}
