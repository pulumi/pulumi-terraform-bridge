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

	"github.com/stretchr/testify/assert"
)

func TestExamplesCache(t *testing.T) {
	t.Parallel()

	hcl := `
		resource "random_password" "password" {
		  length           = 16
		  special          = true
		  override_special = "!#$%&*()-_=+[]{}<>:?"
		}
        `

	ts := `
		import * as pulumi from "@pulumi/pulumi";
		import * as aws from "@pulumi/aws";
		import * as random from "@pulumi/random";

		const password = new random.RandomPassword("password", {
		    length: 16,
		    special: true,
		    overrideSpecial: "!#$%&*()-_=+[]{}<>:?",
		});
	`

	t.Run("enabled", func(t *testing.T) {
		dir := t.TempDir()
		cache := newExamplesCache("ex", "0.0.1", dir)

		_, ok := cache.Lookup(hcl, "typescript")
		assert.False(t, ok)

		cache.Store(hcl, "typescript", ts)

		recall, ok := cache.Lookup(hcl, "typescript")
		assert.True(t, ok)
		assert.Equal(t, ts, recall)
	})

	t.Run("disabled", func(t *testing.T) {
		cache := newExamplesCache("ex", "0.0.1", "")
		assert.False(t, cache.enabled)

		_, ok := cache.Lookup(hcl, "typescript")
		assert.False(t, ok)

		cache.Store(hcl, "typescript", ts)

		_, ok = cache.Lookup(hcl, "typescript")
		assert.False(t, ok)
	})
}
