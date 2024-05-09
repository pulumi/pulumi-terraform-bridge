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

package tfbridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Transforms inputs to enforce input bag conformance with ConflictsWith constraints.
//
// This arbitrarily drops properties until none are left in conflict.
//
// Why is this necessary: Read in Pulumi is expected to produce an input bag that is going to subsequently pass Check,
// but TF providers are not always well-behaved in this regard.
func deconflict(
	ctx context.Context,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*info.Schema,
	inputs resource.PropertyMap,
) resource.PropertyMap {
	cm := newConflictsMap(schemaMap)
	visitedPaths := map[string]struct{}{}
	visit := func(pp resource.PropertyPath, pv resource.PropertyValue) (resource.PropertyValue, error) {
		sp := PropertyPathToSchemaPath(pp, schemaMap, schemaInfos)
		conflict := false
		var conflictAt walk.SchemaPath
		for _, cp := range cm.ConflictingPaths(sp) {
			if _, ok := visitedPaths[cp.MustEncodeSchemaPath()]; ok {
				conflict = true
				conflictAt = cp
				break
			}
		}
		if conflict {
			msg := fmt.Sprintf("Dropping property at %q to respect ConflictsWith constraint from %q",
				pp.String(), conflictAt.MustEncodeSchemaPath())
			GetLogger(ctx).Debug(msg)
			return resource.NewNullProperty(), nil
		}
		visitedPaths[sp.MustEncodeSchemaPath()] = struct{}{}
		return pv, nil
	}

	obj := resource.NewObjectProperty(inputs)
	pv, err := propertyvalue.TransformPropertyValue(resource.PropertyPath{}, visit, obj)
	contract.AssertNoErrorf(err, "deconflict transformation is never expected to fail")
	contract.Assertf(pv.IsObject(), "deconflict transformation is not expected to change objects to something else")
	return pv.ObjectValue()
}

type conflictsMap struct {
	conflicts map[string][]walk.SchemaPath
}

func (cm *conflictsMap) ConflictingPaths(atPath walk.SchemaPath) []walk.SchemaPath {
	return cm.conflicts[atPath.MustEncodeSchemaPath()]
}

func (cm *conflictsMap) AddConflict(atPath walk.SchemaPath, conflictingPaths []walk.SchemaPath) {
	cm.conflicts[atPath.MustEncodeSchemaPath()] = conflictingPaths
}

func newConflictsMap(schemaMap shim.SchemaMap) *conflictsMap {
	cm := &conflictsMap{conflicts: map[string][]walk.SchemaPath{}}
	walk.VisitSchemaMap(schemaMap, func(sp walk.SchemaPath, s shim.Schema) {
		if len(s.ConflictsWith()) > 0 {
			conflictingPaths := []walk.SchemaPath{}
			for _, p := range s.ConflictsWith() {
				conflictingPaths = append(conflictingPaths, parseConflictsWith(p))
			}
			cm.AddConflict(sp, conflictingPaths)
		}
	})
	return cm
}

func parseConflictsWith(s string) walk.SchemaPath {
	parts := strings.Split(s, ".")
	result := walk.NewSchemaPath()
	for _, p := range parts {
		_, strconvErr := strconv.Atoi(p)
		isNum := strconvErr == nil
		if isNum {
			result = result.Element()
		} else {
			result = result.GetAttr(p)
		}
	}
	return result
}
