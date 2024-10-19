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
package crosstests

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Adapted from diff_check.go
type inputTestCase struct {
	// Schema for the resource under test
	Resource *schema.Resource

	Config     any
	ObjectType *tftypes.Object

	DisablePlanResourceChange bool
}

// Adapted from diff_check.go
//
// Deprecated: [Create] should be used for new tests.
func runCreateInputCheck(t T, tc inputTestCase) {
	if tc.Resource.CreateContext != nil {
		t.Errorf("Create methods should not be set for these tests!")
	}

	Create(t,
		tc.Resource.Schema,
		coalesceInputs(t, tc.Resource.Schema, tc.Config),
		InferPulumiValue(),
		CreateStateUpgrader(tc.Resource.SchemaVersion, tc.Resource.StateUpgraders),
		CreateTimeout(tc.Resource.Timeouts),
	)
}
