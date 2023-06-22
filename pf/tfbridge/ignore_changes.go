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
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func applyIgnoreChanges(old, new resource.PropertyMap, ignoreChanges []string) (resource.PropertyMap, error) {
	var paths []resource.PropertyPath
	var errs []error
	for i, p := range ignoreChanges {
		pp, err := resource.ParsePropertyPath(p)
		if err != nil {
			errs = append(errs,
				fmt.Errorf("failed to parse property path %d: %s", i, p))
			continue
		}
		paths = append(paths, pp)
	}
	if len(errs) > 0 {
		return nil, &multierror.Error{Errors: errs}
	}

	newValue := resource.NewObjectProperty(new.Copy())
	oldValue := resource.NewObjectProperty(old)
	for _, p := range paths {

		// Its not 100% clear on what an empty property path means at this point.
		//
		// applyIgnoreChanges will treat an empty path as fully resolved, so we
		// don't want to pass that as a top level action.
		if len(p) == 0 {
			continue
		}
		newValue = applyIgnorePath(p, oldValue, newValue)
	}
	return newValue.ObjectValue(), nil
}

// Apply a ignoreChanges property path by copying the element from src to dst.
func applyIgnorePath(p resource.PropertyPath, src, dst resource.PropertyValue) resource.PropertyValue {
	if len(p) == 0 {
		// The path is exhausted, which means that src and dst are the elements
		// the path points at. We return src.
		return src
	}

	switch part := p[0].(type) {
	case string:
		if part == "*" {
			// If the object has "*" as a genuine key, we can't recurse
			// normally, because that would replace ("*" :: rest) with ("*" ::
			// rest), overflowing the stack.
			//
			// Instead we ignore that key, but avoid exiting early. The "*"
			// key will be handled normally by the rest of the function.
			var objectHasGlobKey bool

			switch {
			case src.IsArray() && dst.IsArray():
				for i := 0; i < len(src.ArrayValue()); i++ {
					dst = applyIgnorePath(
						append(resource.PropertyPath{i},
							p[1:]...), src, dst)
				}
			case src.IsObject() && dst.IsObject():
				keys := map[string]struct{}{}
				for k := range src.ObjectValue() {
					keys[string(k)] = struct{}{}
				}
				for k := range dst.ObjectValue() {
					keys[string(k)] = struct{}{}
				}

				for k := range keys {
					if k == "*" {
						objectHasGlobKey = true
						continue
					}
					dst = applyIgnorePath(
						append(resource.PropertyPath{k},
							p[1:]...), src, dst)
				}
			}
			if !objectHasGlobKey {
				return dst
			}
		}

		if !src.IsObject() || !dst.IsObject() {
			return dst
		}

		vSrc, okSrc := src.ObjectValue()[resource.PropertyKey(part)]
		vDst, okDst := dst.ObjectValue()[resource.PropertyKey(part)]

		// If we are able to access the element in the destination path, but not
		// the source path and this is the last element in the chain, this
		// operates as a delete.
		//
		// Consider the example:
		//
		//   ignoreChanges: [ "path" ]
		//   old: { "other": 0 }
		//   new: { "other": 0, "path": 42 }
		//
		// Here we would delete "path" from `new`, propagating the absence from
		// `old`.
		if okDst && len(p) == 1 && !okSrc {
			delete(dst.ObjectValue(), resource.PropertyKey(part))
			return dst
		}

		// We need to handle the inverse of the above case, preserving old
		// elements when the new element is dropped. The example here would be
		//
		//   ignoreChanges: [ "path" ]
		//   old: { "other": 0, "path": 42 }
		//   new: { "other": 0 }
		//
		// Again we would need to add the `old["path"]` segment to `new`.
		if okSrc && len(p) == 1 && !okDst {
			dst.ObjectValue()[resource.PropertyKey(part)] = vSrc
			return dst
		}

		// If we are not able to access the relevant element in both map, and we
		// didn't hit any of the above special cases, then the path is invalid and
		// we don't apply it.
		if !okSrc || !okDst {
			return dst
		}

		obj := dst.ObjectValue()
		obj[resource.PropertyKey(part)] = applyIgnorePath(p[1:], vSrc, vDst)

		return resource.NewObjectProperty(obj)

	case int:
		// If we are not able to access the relevant element in both arrays, then
		// the path is invalid, and we do not apply it.
		if !src.IsArray() || !dst.IsArray() {
			return dst
		}
		srcArr, dstArr := src.ArrayValue(), dst.ArrayValue()
		if part >= len(srcArr) || part >= len(dstArr) {
			return dst
		}

		dstArr[part] = applyIgnorePath(p[1:], srcArr[part], dstArr[part])
		return dst

	default:
		msg := fmt.Sprintf(
			"invalid property path element: expected a string or int, found %T: %[1]v", part)
		panic(msg)
	}
}
