// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
)

// containsComputedValues returns true if the given property value is or contains a computed value.
func containsComputedValues(v resource.PropertyValue) bool {
	switch {
	case v.IsArray():
		for _, e := range v.ArrayValue() {
			if containsComputedValues(e) {
				return true
			}
		}
		return false
	case v.IsObject():
		for _, e := range v.ObjectValue() {
			if containsComputedValues(e) {
				return true
			}
		}
		return false
	default:
		return v.IsComputed() || v.IsOutput()
	}
}

// A propertyVisitor is called for each property value in `visitPropertyValue` with the TF attribute key, Pulumi
// property name, and the value itself. If the visitor returns `false`, `visitPropertyValue` will not recurse into
// the value.
type propertyVisitor func(attributeKey, propertyPath string, value resource.PropertyValue) bool

// visitPropertyValue checks the given property for diffs and invokes the given callback if a diff is found.
//
// This function contains the core logic used to convert a terraform.InstanceDiff to a list of Pulumi property paths
// that have changed and the type of change for each path.
//
// If not for the presence of sets and the fact that they are mapped to Pulumi array properties, this process would be
// rather straightforward: we could loop over each path in the instance diff, convert its path to a Pulumi property
// path, and convert the diff kind to a Pulumi diff kind. Unfortunately, the set mapping complicates this process, as
// the array index that corresponds to a set element cannot be derived from the set element's path: part of the path of
// a set element is the element's hash code, and we cannot derive the corresponding array element's index from this
// hash code.
//
// To solve this problem, we recurse through each element in the given property value, compute its path, and
// check to see if the InstanceDiff has an entry for that path.
func visitPropertyValue(name, path string, v resource.PropertyValue, tfs shim.Schema, ps *SchemaInfo,
	rawNames bool, visitor propertyVisitor) {

	if IsMaxItemsOne(tfs, ps) {
		if v.IsNull() {
			v = resource.NewArrayProperty([]resource.PropertyValue{})
		} else {
			v = resource.NewArrayProperty([]resource.PropertyValue{v})
		}
	}

	if !visitor(name, path, v) {
		return
	}

	switch {
	case v.IsArray():
		isset := tfs != nil && tfs.Type() == shim.TypeSet

		etfs, eps := elemSchemas(tfs, ps)

		for i, e := range v.ArrayValue() {
			ep := path
			if !IsMaxItemsOne(tfs, ps) {
				ep = fmt.Sprintf("%s[%d]", path, i)
			}

			ti := strconv.FormatInt(int64(i), 10)
			if isset {
				// Convert the element into its TF representation and hash it. This is necessary because the hash value
				// forms part of the key. We round-trip through a config field reader so that TF has the opportunity to
				// fill in default values for empty fields (note that this is a property of the field reader, not of
				// the schema) as it does when computing the hash code for a set element.
				ctx := &conversionContext{}
				ev, err := ctx.MakeTerraformInput(ep, resource.PropertyValue{}, e, etfs, eps, rawNames)
				if err != nil {
					return
				}

				if !e.IsComputed() && !e.IsOutput() {
					if ev, err = tfs.SetElement(makeConfig(ev)); err != nil {
						return
					}
				}

				ti = strconv.FormatInt(int64(tfs.SetHash(ev)), 10)
				if containsComputedValues(e) {
					// TF adds a '~' prefix to the hash code for any set element that contains computed values.
					ti = "~" + ti
				}
			}

			en := name + "." + ti
			visitPropertyValue(en, ep, e, etfs, eps, rawNames, visitor)
		}
	case v.IsObject():
		var tfflds shim.SchemaMap
		if tfs != nil {
			if res, isres := tfs.Elem().(shim.Resource); isres {
				tfflds = res.Schema()
			}
		}
		var psflds map[string]*SchemaInfo
		if ps != nil {
			psflds = ps.Fields
		}

		rawElementNames := rawNames || useRawNames(tfs)
		for k, e := range v.ObjectValue() {
			var elementPath string
			if strings.ContainsAny(string(k), `."[]`) {
				elementPath = fmt.Sprintf(`%s.["%s"]`, path, strings.ReplaceAll(string(k), `"`, `\"`))
			} else {
				elementPath = fmt.Sprintf("%s.%s", path, k)
			}

			en, etf, eps := getInfoFromPulumiName(k, tfflds, psflds, rawElementNames)
			visitPropertyValue(name+"."+en, elementPath, e, etf, eps, rawElementNames, visitor)
		}
	}
}

func makePropertyDiff(name, path string, v resource.PropertyValue, tfDiff shim.InstanceDiff,
	diff map[string]*pulumirpc.PropertyDiff, tfs shim.Schema, ps *SchemaInfo, finalize, rawNames bool) {

	visitor := func(name, path string, v resource.PropertyValue) bool {
		recurse := false
		switch {
		case v.IsArray():
			// If this value has a diff and is considered computed by Terraform, the diff will be woefully incomplete. In
			// this case, do not recurse into the array; instead, just use the count diff for the details.
			d := tfDiff.Attribute(name + ".#")
			if d == nil {
				return true
			}
			name += ".#"
			recurse = !d.NewComputed
		case v.IsObject():
			// If this value has a diff and is considered computed by Terraform, the diff will be woefully incomplete. In
			// this case, do not recurse into the array; instead, just use the count diff for the details.
			d := tfDiff.Attribute(name + ".%")
			if d == nil {
				return true
			}
			name += ".%"
			recurse = !d.NewComputed
		case v.IsComputed() || v.IsOutput():
			// If this is a computed value, it may be replacing a map or list. To detect that case, check for attribute
			// diffs at the various count paths and update `name` appropriately.
			switch {
			case tfDiff.Attribute(name) != nil:
				// We have a diff at this name; process it as usual.
			case tfDiff.Attribute(name+".#") != nil:
				// We have a diff at the list count. Use that name when deciding on the diff kind below.
				name += ".#"
			case tfDiff.Attribute(name+".%") != nil:
				// We have a diff at the map or set count. Use that name when deciding on the diff kind below.
				name += ".%"
			}
		}
		if d := tfDiff.Attribute(name); d != nil && d.Old != d.New {
			other, hasOtherDiff := diff[path]

			// If we're finalizing the diff, we want to remove any ADD diffs that were only present in the state.
			// These diffs are typically changes to output properties that we don't care about.
			if finalize {
				if hasOtherDiff &&
					(other.Kind == pulumirpc.PropertyDiff_ADD || other.Kind == pulumirpc.PropertyDiff_ADD_REPLACE) {
					delete(diff, path)
				}
				return false
			}

			var kind pulumirpc.PropertyDiff_Kind
			switch {
			case d.NewRemoved:
				if d.RequiresNew {
					kind = pulumirpc.PropertyDiff_DELETE_REPLACE
				} else {
					kind = pulumirpc.PropertyDiff_DELETE
				}
			case !hasOtherDiff:
				if d.RequiresNew {
					kind = pulumirpc.PropertyDiff_ADD_REPLACE
				} else {
					kind = pulumirpc.PropertyDiff_ADD
				}
			default:
				if d.RequiresNew {
					kind = pulumirpc.PropertyDiff_UPDATE_REPLACE
				} else {
					kind = pulumirpc.PropertyDiff_UPDATE
				}
			}
			diff[path] = &pulumirpc.PropertyDiff{Kind: kind}
		}
		return recurse
	}

	visitPropertyValue(name, path, v, tfs, ps, rawNames, visitor)
}

func doIgnoreChanges(tfs shim.SchemaMap, ps map[string]*SchemaInfo, olds, news resource.PropertyMap,
	ignoredPaths []string, tfDiff shim.InstanceDiff) {

	if tfDiff == nil {
		return
	}

	ignoredPathSet := map[string]bool{}
	for _, p := range ignoredPaths {
		ignoredPathSet[p] = true
	}

	ignoredKeySet := map[string]bool{}
	visitor := func(attributeKey, propertyPath string, _ resource.PropertyValue) bool {
		if ignoredPathSet[propertyPath] {
			ignoredKeySet[attributeKey] = true
		}
		return true
	}
	for k, v := range olds {
		en, etf, eps := getInfoFromPulumiName(k, tfs, ps, false)
		visitPropertyValue(en, string(k), v, etf, eps, useRawNames(etf), visitor)
	}
	for k, v := range news {
		en, etf, eps := getInfoFromPulumiName(k, tfs, ps, false)
		visitPropertyValue(en, string(k), v, etf, eps, useRawNames(etf), visitor)
	}

	tfDiff.IgnoreChanges(ignoredKeySet)
}

// makeDetailedDiff converts the given state (olds), config (news), and InstanceDiff to a Pulumi property diff.
//
// See makePropertyDiff for more details.
func makeDetailedDiff(tfs shim.SchemaMap, ps map[string]*SchemaInfo, olds, news resource.PropertyMap,
	tfDiff shim.InstanceDiff) map[string]*pulumirpc.PropertyDiff {

	if tfDiff == nil {
		return map[string]*pulumirpc.PropertyDiff{}
	}

	// Check both the old state and the new config for diffs and report them as necessary.
	//
	// There is a minor complication here: Terraform has no concept of an "add" diff. Instead, adds are recorded as
	// updates with an old property value of the empty string. In order to detect adds--and to ensure that all diffs in
	// the InstanceDiff are reflected in the resulting Pulumi property diff--we first call this function with each
	// property in a resource's state, then with each property in its config. Any diffs that only appear in the config
	// are treated as adds; diffs that appear in both the state and config are treated as updates.
	diff := map[string]*pulumirpc.PropertyDiff{}
	for k, v := range olds {
		en, etf, eps := getInfoFromPulumiName(k, tfs, ps, false)
		makePropertyDiff(en, string(k), v, tfDiff, diff, etf, eps, false, useRawNames(etf))
	}
	for k, v := range news {
		en, etf, eps := getInfoFromPulumiName(k, tfs, ps, false)
		makePropertyDiff(en, string(k), v, tfDiff, diff, etf, eps, false, useRawNames(etf))
	}
	for k, v := range olds {
		en, etf, eps := getInfoFromPulumiName(k, tfs, ps, false)
		makePropertyDiff(en, string(k), v, tfDiff, diff, etf, eps, true, useRawNames(etf))
	}
	return diff
}
