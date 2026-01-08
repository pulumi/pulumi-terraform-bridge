// Copyright 2016-2025, Pulumi Corporation.
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

package schemashim

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
)

// TestBlockOptionalityWithValidator tests that blocks with SizeBetween validators
// are still marked as optional, not required.
// This test addresses: https://github.com/pulumi/pulumi-terraform-bridge/issues/3080
func TestBlockOptionalityWithValidator(t *testing.T) {
	t.Parallel()
	// Create a ListNestedBlock with a SizeBetween(1, 2) validator
	// This mimics aws_bedrockagentcore_gateway's interceptor_configuration block
	block := rschema.ListNestedBlock{
		NestedObject: rschema.NestedBlockObject{
			Attributes: map[string]rschema.Attribute{
				"test_field": rschema.StringAttribute{
					Required: true,
				},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeBetween(1, 2),
		},
	}

	// Convert to our internal representation
	bridgeBlock := pfutils.FromResourceBlock(block)
	schema := &blockSchema{
		key:   "test_block",
		block: bridgeBlock,
	}

	// In TF Plugin Framework, blocks are always optional by default.
	// The SizeBetween(1, 2) validator only constrains the list size
	// when the block IS provided - it does not make the block required.
	assert.True(t, schema.Optional(), "Block should be optional")
	assert.False(t, schema.Required(), "Block should not be required")

	// However, MinItems should still reflect the validator's constraint
	// for informational purposes (used for other logic like flattening)
	assert.Equal(t, 1, schema.MinItems(), "MinItems should be 1 from validator")
	assert.Equal(t, 2, schema.MaxItems(), "MaxItems should be 2 from validator")
}

// TestBlockOptionalityWithoutValidator tests that blocks without validators
// are also marked as optional.
func TestBlockOptionalityWithoutValidator(t *testing.T) {
	t.Parallel()
	// Create a ListNestedBlock without any validators
	block := rschema.ListNestedBlock{
		NestedObject: rschema.NestedBlockObject{
			Attributes: map[string]rschema.Attribute{
				"test_field": rschema.StringAttribute{
					Optional: true,
				},
			},
		},
	}

	bridgeBlock := pfutils.FromResourceBlock(block)
	schema := &blockSchema{
		key:   "test_block",
		block: bridgeBlock,
	}

	assert.True(t, schema.Optional(), "Block without validators should be optional")
	assert.False(t, schema.Required(), "Block without validators should not be required")
	assert.Equal(t, 0, schema.MinItems(), "MinItems should be 0 without validators")
	assert.Equal(t, 0, schema.MaxItems(), "MaxItems should be 0 without validators")
}

// TestSingleNestedBlockOptionality tests that SingleNestedBlock is also optional.
func TestSingleNestedBlockOptionality(t *testing.T) {
	t.Parallel()
	block := rschema.SingleNestedBlock{
		Attributes: map[string]rschema.Attribute{
			"test_field": rschema.StringAttribute{
				Required: true,
			},
		},
	}

	bridgeBlock := pfutils.FromResourceBlock(block)
	schema := &blockSchema{
		key:   "test_block",
		block: bridgeBlock,
	}

	assert.True(t, schema.Optional(), "SingleNestedBlock should be optional")
	assert.False(t, schema.Required(), "SingleNestedBlock should not be required")
}

// TestBlockRequiredWithValidator tests that blocks with SizeBetween validators
// are still marked as optional, not required.
// This test addresses: https://github.com/pulumi/pulumi-terraform-bridge/issues/3080
func TestBlockRequiredWithValidator(t *testing.T) {
	t.Parallel()
	// Create a ListNestedBlock with a SizeBetween(1, 2) validator and an IsRequired() validator.
	// This mimics aws_bedrockagentcore_gateway's interceptor_configuration block
	block := rschema.ListNestedBlock{
		NestedObject: rschema.NestedBlockObject{
			Attributes: map[string]rschema.Attribute{
				"test_field": rschema.StringAttribute{
					Required: true,
				},
			},
		},
		Validators: []validator.List{
			listvalidator.IsRequired(),
			listvalidator.SizeBetween(1, 2),
		},
	}

	// Convert to our internal representation
	bridgeBlock := pfutils.FromResourceBlock(block)
	schema := &blockSchema{
		key:   "test_block",
		block: bridgeBlock,
	}

	// In TF Plugin Framework, blocks are always optional by default.
	// The SizeBetween(1, 2) validator only constrains the list size
	// when the block IS provided - it does not make the block required.
	assert.False(t, schema.Optional(), "Block should not be optional")
	assert.True(t, schema.Required(), "Block should be required")

	// However, MinItems should still reflect the validator's constraint
	// for informational purposes (used for other logic like flattening)
	assert.Equal(t, 1, schema.MinItems(), "MinItems should be 1 from validator")
	assert.Equal(t, 2, schema.MaxItems(), "MaxItems should be 2 from validator")
}

// TestSetRequiredNestedBlock tests that a set block can be required.
func TestSetRequiredNestedBlock(t *testing.T) {
	t.Parallel()
	block := rschema.SetNestedBlock{
		NestedObject: rschema.NestedBlockObject{
			Attributes: map[string]rschema.Attribute{
				"test_field": rschema.StringAttribute{
					Required: true,
				},
			},
		},
		Validators: []validator.Set{
			setvalidator.IsRequired(),
		},
	}

	bridgeBlock := pfutils.FromResourceBlock(block)
	schema := &blockSchema{
		key:   "test_block",
		block: bridgeBlock,
	}

	assert.False(t, schema.Optional(), "Block should not be optional")
	assert.True(t, schema.Required(), "Block should be required")
}

// TestSingleRequiredNestedBlock tests that a single nested block can be required.
func TestSingleRequiredNestedBlock(t *testing.T) {
	t.Parallel()
	block := rschema.SingleNestedBlock{
		Attributes: map[string]rschema.Attribute{
			"test_field": rschema.StringAttribute{
				Required: true,
			},
		},
		Validators: []validator.Object{
			objectvalidator.IsRequired(),
		},
	}

	bridgeBlock := pfutils.FromResourceBlock(block)
	schema := &blockSchema{
		key:   "test_block",
		block: bridgeBlock,
	}

	assert.False(t, schema.Optional(), "Block should not be optional")
	assert.True(t, schema.Required(), "Block should be required")
}
