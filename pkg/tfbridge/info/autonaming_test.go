// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package info

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestComputeAutoNameDefault(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("basic", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN: resource.URN("urn:pulumi:stack::project::type::name"),
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, "name", result)
	})

	t.Run("with separator and random suffix", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Separator: "-",
			Randlen:   4,
		}, opts)
		assert.NoError(t, err)
		assert.Regexp(t, "^name-[0-9a-f]{4}$", result)
	})

	t.Run("respects prior state", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN: resource.URN("urn:pulumi:stack::project::type::name"),
			PriorState: resource.PropertyMap{
				"name": resource.NewStringProperty("existing-name"),
			},
			PriorValue: resource.NewStringProperty("existing-name"),
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, "existing-name", result)
	})

	t.Run("propose mode", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "proposed-name",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, "proposed-name", result)
	})

	t.Run("propose mode with transform", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "proposed-name",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Transform: func(s string) string {
				return s + "-transformed"
			},
		}, opts)
		assert.NoError(t, err)
		assert.Equal(t, "proposed-name-transformed", result)
	})

	t.Run("propose mode with maxlen", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "this-is-a-very-long-proposed-name",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		_, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Maxlen: 10,
		}, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum length")
	})

	t.Run("propose mode with charset", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "name-123",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		_, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Charset: []rune("abcdefghijklmnopqrstuvwxyz-"),
		}, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "contains invalid character")
	})

	t.Run("propose mode ignores separator if no charset specified", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "name-with-dashes",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Separator: "_",
		}, opts)
		assert.NoError(t, err)
		assert.Equal(t, "name-with-dashes", result)
	})

	t.Run("propose mode with separator replacement and charset", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "name-with_mixed-separators",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Separator: ".",
			Charset:   []rune("abcdefghijklmnopqrstuvwxyz."),
		}, opts)
		assert.NoError(t, err)
		assert.Equal(t, "name.with.mixed.separators", result)
	})

	t.Run("propose mode with separator in charset", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "name-with-dashes",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Separator: "-",
			Charset:   []rune("abcdefghijklmnopqrstuvwxyz-"),
		}, opts)
		assert.NoError(t, err)
		// Should preserve dashes since they're in the charset
		assert.Equal(t, "name-with-dashes", result)
	})

	t.Run("propose mode with mixed separators and partial charset", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "name-with_mixed-separators",
				Mode:         ComputeDefaultAutonamingModePropose,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			Separator: "+",
			// Only include - in charset, _ should still be replaced
			Charset: []rune("abcdefghijklmnopqrstuvwxyz+-"),
		}, opts)
		assert.NoError(t, err)
		// Should preserve - but replace _ with +
		assert.Equal(t, "name-with+mixed-separators", result)
	})

	t.Run("enforce mode", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "proposed-name",
				Mode:         ComputeDefaultAutonamingModeEnforce,
			},
		}

		result, err := ComputeAutoNameDefault(ctx, AutoNameOptions{
			// All of these options are ignored by design when mode is enforce.
			Transform: func(s string) string {
				return s + "-transformed"
			},
			PostTransform: func(res *PulumiResource, s string) (string, error) {
				return s + "-posttransformed", nil
			},
			Maxlen:    5,
			Charset:   []rune("abc"),
			Separator: "_",
		}, opts)
		assert.NoError(t, err)
		// In enforce mode, the transform should be ignored and proposed name used exactly
		assert.Equal(t, "proposed-name", result)
	})

	t.Run("disable mode", func(t *testing.T) {
		opts := ComputeDefaultOptions{
			URN:  resource.URN("urn:pulumi:stack::project::type::name"),
			Seed: []byte("test-seed"),
			Autonaming: &ComputeDefaultAutonamingOptions{
				ProposedName: "proposed-name",
				Mode:         ComputeDefaultAutonamingModeDisable,
			},
		}

		_, err := ComputeAutoNameDefault(ctx, AutoNameOptions{}, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "automatic naming is disabled")
	})
}
