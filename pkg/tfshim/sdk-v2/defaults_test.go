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

package sdkv2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// Regressing an issue in the wild where a default value is given that does not validate. Currently pkg/tfbridge may
// call DefaultValue() at runtime, but ignores Validators. In the case of invalid defaults, it may send an invalid value
// to the provider, which will run validators and fail. The quick fix applied in defaults.go tested here is to detect
// invalid defaults and drop them, pretending they were never set.
//
// See pulumi/pulumi-terraform-bridge#720
func TestDroppingInvalidDefaults(t *testing.T) {
	suspectSchema := sdkv2.NewSchema(&schema.Schema{
		Type:         schema.TypeInt,
		Optional:     true,
		Default:      -1,
		ValidateFunc: validation.IntBetween(0, 99999),
	})
	dv, err := suspectSchema.DefaultValue()
	assert.Nil(t, dv)
	assert.NoError(t, err)

	goodSchema := sdkv2.NewSchema(&schema.Schema{
		Type:         schema.TypeInt,
		Optional:     true,
		Default:      -1,
		ValidateFunc: validation.IntBetween(-1, 99999),
	})
	dv, err = goodSchema.DefaultValue()
	assert.Equal(t, -1, dv)
	assert.NoError(t, err)

}
