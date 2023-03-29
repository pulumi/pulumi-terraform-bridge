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

package pfutils

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type Diff struct {
	tftypes.ValueDiff
	Type tftypes.Type
}

func (d Diff) Unknown() *tftypes.Value {
	v := tftypes.NewValue(d.Type, tftypes.UnknownValue)
	return &v
}

func (d Diff) Null() *tftypes.Value {
	v := tftypes.NewValue(d.Type, nil)
	return &v
}

// Configures how to join two values.
type JoinOptions struct {
	// Reconciles two values that are not both present or else present but not Equal. At least one of the values is
	// immediate. Immediate values are nulls, unknowns, and scalar values (not List, Set, Map, Object).
	Reconcile func(Diff) (*tftypes.Value, error)

	// Optional. Allows overriding how set elements are compared for equality.
	SetElementEqual Eq
}

// Joins two values. The missing combinator from tftypes, which provides Diff and Walk but no ability to join.
func Join(opts JoinOptions, path *tftypes.AttributePath, x, y *tftypes.Value) (*tftypes.Value, error) {
	o := &opts
	if isImmediate(x) || isImmediate(y) {
		return joinImmediate(*o, path, x, y)
	}
	return joinCollections(*o, path, *x, *y)
}

func isImmediate(x *tftypes.Value) bool {
	if x == nil {
		return true
	}
	if x.IsNull() {
		return true
	}
	if !x.IsKnown() {
		return true
	}
	ty := x.Type()
	switch {
	case ty.Is(tftypes.Object{}),
		ty.Is(tftypes.Map{}),
		ty.Is(tftypes.Tuple{}),
		ty.Is(tftypes.Set{}),
		ty.Is(tftypes.List{}):
		return false
	}
	return true
}

func joinImmediate(opts JoinOptions, path *tftypes.AttributePath, x, y *tftypes.Value) (*tftypes.Value, error) {
	switch {
	case x == nil && y == nil:
		return nil, nil
	case x != nil && y != nil && x.Equal(*y):
		return x, nil
	default:
		if x != nil && y != nil && !x.Type().Equal(y.Type()) {
			return nil, fmt.Errorf("Join can only work on values of identical types")
		}
		var t tftypes.Type
		if x != nil {
			t = x.Type()
		}
		if y != nil {
			t = y.Type()
		}
		return opts.Reconcile(Diff{
			Type: t,
			ValueDiff: tftypes.ValueDiff{
				Path:   path,
				Value1: x,
				Value2: y,
			},
		})
	}
}

func joinCollections(opts JoinOptions, path *tftypes.AttributePath, x, y tftypes.Value) (*tftypes.Value, error) {
	if !x.Type().Equal(y.Type()) {
		return nil, fmt.Errorf("Join can only work on values of identical types")
	}
	ty := x.Type()
	switch {
	case ty.Is(tftypes.List{}):
		elementType := func(int) tftypes.Type {
			return ty.(tftypes.List).ElementType
		}
		z, err := joinSequences(opts, path, elementType, x, y)
		if err != nil {
			return nil, err
		}
		res := tftypes.NewValue(ty, z)
		return &res, nil
	case ty.Is(tftypes.Set{}):
		z, err := joinSets(opts, path, x, y)
		if err != nil {
			return nil, err
		}
		res := tftypes.NewValue(ty, z)
		return &res, nil
	case ty.Is(tftypes.Tuple{}):
		elementType := func(i int) tftypes.Type {
			return ty.(tftypes.Tuple).ElementTypes[i]
		}
		z, err := joinSequences(opts, path, elementType, x, y)
		if err != nil {
			return nil, err
		}
		res := tftypes.NewValue(ty, z)
		return &res, nil
	case ty.Is(tftypes.Map{}):
		z, err := joinDicts(func(key string, x, y *tftypes.Value) (*tftypes.Value, error) {
			return Join(opts, path.WithElementKeyString(key), x, y)
		}, y, y)
		if err != nil {
			return nil, err
		}
		return &z, nil
	case ty.Is(tftypes.Object{}):
		objTy := ty.(tftypes.Object)
		z, err := joinDicts(func(key string, x, y *tftypes.Value) (*tftypes.Value, error) {
			joined, err := Join(opts, path.WithAttributeName(key), x, y)
			if err != nil {
				return nil, err
			}
			if joined == nil {
				_, optional := objTy.OptionalAttributes[key]
				if optional {
					return nil, nil
				}
				t := objTy.AttributeTypes[key]
				typedNil := tftypes.NewValue(t, nil)
				return &typedNil, nil
			}
			return joined, nil
		}, x, y)
		if err != nil {
			return nil, err
		}
		return &z, nil
	default:
		panic("joinCollections: impossible case")
	}
}

func joinSequences(opts JoinOptions, path *tftypes.AttributePath, elementType func(int) tftypes.Type,
	x, y tftypes.Value) ([]tftypes.Value, error) {
	var xs, ys []tftypes.Value
	if err := x.As(&xs); err != nil {
		return nil, err
	}
	if err := y.As(&ys); err != nil {
		return nil, err
	}

	var joined []tftypes.Value

	n := len(xs)
	if len(ys) > n {
		n = len(ys)
	}

	for i := 0; i < n; i++ {
		var xe, ye *tftypes.Value
		if i < len(xs) {
			xe = &xs[i]
		}
		if i < len(ys) {
			ye = &ys[i]
		}
		j, err := Join(opts, path.WithElementKeyInt(i), xe, ye)
		if err != nil {
			return nil, err
		}
		if j != nil {
			joined = append(joined, *j)
		}
	}

	return joined, nil
}

func joinSets(opts JoinOptions, path *tftypes.AttributePath, x, y tftypes.Value) ([]tftypes.Value, error) {
	equal := DefaultEq
	if opts.SetElementEqual != nil {
		equal = opts.SetElementEqual
	}

	var xs, ys []tftypes.Value
	if err := x.As(&xs); err != nil {
		return nil, err
	}
	if err := y.As(&ys); err != nil {
		return nil, err
	}
	var joined []tftypes.Value
	var seen []tftypes.Value
	for _, v := range append(xs, ys...) {
		subpath := path.WithElementKeyValue(v)
		if !contains(equal, subpath, seen, v) {
			seen = append(seen, v)
			xe, err := tryFind(equal, subpath, xs, v)
			if err != nil {
				return nil, err
			}
			ye, err := tryFind(equal, subpath, ys, v)
			if err != nil {
				return nil, err
			}
			j, err := Join(opts, subpath, xe, ye)
			if err != nil {
				return nil, err
			}
			if j != nil && !contains(equal, subpath, joined, *j) {
				joined = append(joined, *j)
			}
		}
	}
	return joined, nil
}

func tryFind(equal Eq, p *tftypes.AttributePath, set []tftypes.Value, key tftypes.Value) (*tftypes.Value, error) {
	for _, j := range set {
		eq, err := equal.Equal(p, j, key)
		if err != nil {
			return nil, err
		}
		if eq {
			return &j, nil
		}
	}
	return nil, nil
}

func contains(equal Eq, p *tftypes.AttributePath, set []tftypes.Value, v tftypes.Value) bool {
	found, _ := tryFind(equal, p, set, v)
	return found != nil
}

func joinDicts(joinElement func(key string, x, y *tftypes.Value) (*tftypes.Value, error),
	x, y tftypes.Value) (tftypes.Value, error) {
	var xs, ys map[string]tftypes.Value
	if err := x.As(&xs); err != nil {
		return tftypes.Value{}, err
	}
	if err := y.As(&ys); err != nil {
		return tftypes.Value{}, err
	}
	allKeys := map[string]struct{}{}
	for k := range xs {
		allKeys[k] = struct{}{}
	}
	for k := range ys {
		allKeys[k] = struct{}{}
	}
	joined := map[string]tftypes.Value{}
	for k := range allKeys {
		var xe, ye *tftypes.Value
		if xv, gotX := xs[k]; gotX {
			xe = &xv
		}
		if yv, gotY := ys[k]; gotY {
			ye = &yv
		}
		j, err := joinElement(k, xe, ye)
		if err != nil {
			return tftypes.Value{}, err
		}
		if j != nil {
			joined[k] = *j
		}
	}
	return tftypes.NewValue(x.Type(), joined), nil
}
