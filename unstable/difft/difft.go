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

package difft

import (
	"fmt"
)

type Change int

const (
	Insert Change = iota
	Remove
	Keep
)

type Edit[T any] struct {
	Change  Change
	Element T
}

func (e Edit[T]) String() string {
	switch e.Change {
	case Insert:
		return fmt.Sprintf("insert[%v]", e.Element)
	case Remove:
		return fmt.Sprintf("remove[%v]", e.Element)
	default:
		return fmt.Sprintf("keep[%v]", e.Element)
	}
}

type Edits[T any] []Edit[T]

func (edits Edits[T]) Apply(xs []T) ([]T, error) {
	return edits.apply(xs, nil /* equals checks are only for testing */)
}

func (edits Edits[T]) apply(xs []T, equals func(T, T) bool) ([]T, error) {
	remainder := xs
	var output []T
	for _, ed := range edits {
		switch ed.Change {
		case Keep:
			if len(remainder) == 0 {
				return nil, fmt.Errorf("edits do not apply")
			}
			element := remainder[0]
			remainder = remainder[1:]
			output = append(output, ed.Element)
			if equals != nil && !equals(ed.Element, element) {
				return nil, fmt.Errorf("edits are inconsistent with the original")
			}
		case Insert:
			output = append(output, ed.Element)
		case Remove:
			if len(remainder) == 0 {
				return nil, fmt.Errorf("edits do not apply")
			}
			element := remainder[0]
			remainder = remainder[1:]
			if equals != nil && !equals(ed.Element, element) {
				return nil, fmt.Errorf("wanted to remove %v but seeing %v", ed.Element, element)
			}
		}
	}
	return output, nil
}

type DiffTOptions[T any] struct {
	Equals func(T, T) bool
}

func DiffT[T any](xs, ys []T, opts DiffTOptions[T]) Edits[T] {
	eq := opts.Equals
	if eq == nil {
		eq = func(x T, y T) bool {
			return any(x) == any(y)
		}
	}
	d := &differ[T]{opts.Equals, xs, ys}
	return d.diff()
}

type differ[T any] struct {
	eq func(T, T) bool
	xs []T
	ys []T
}

func (d *differ[T]) difflen() *matrix {
	difflen := newMatrix(len(d.xs)+1, len(d.ys)+1)
	for xp := len(d.xs); xp >= 0; xp-- {
		for yp := len(d.ys); yp >= 0; yp-- {
			l, _ := d.choose(difflen, xp, yp)
			difflen.set(xp, yp, l)
		}
	}
	return difflen
}

func (d *differ[T]) choose(difflen *matrix, xp, yp int) (int, Change) {
	xrem := len(d.xs) - xp
	yrem := len(d.ys) - yp
	switch {
	case xrem == 0:
		return yrem, Insert
	case yrem == 0:
		return xrem, Remove
	}
	l := 1 + difflen.get(xp+1, yp)
	c := Remove
	if n := 1 + difflen.get(xp, yp+1); n < l {
		l = n
		c = Insert
	}
	if d.eq(d.xs[xp], d.ys[yp]) {
		if n := difflen.get(xp+1, yp+1); n < l {
			l = n
			c = Keep
		}
	}
	return l, c
}

func (d *differ[T]) diff() []Edit[T] {
	var edits []Edit[T]
	difflen, xs, ys := d.difflen(), d.xs, d.ys
	for {
		if len(xs) == 0 {
			for _, y := range ys {
				edits = append(edits, d.insert(y))
			}
			return edits
		}
		if len(ys) == 0 {
			for _, x := range xs {
				edits = append(edits, d.remove(x))
			}
			return edits
		}
		xp, yp := len(d.xs)-len(xs), len(d.ys)-len(ys)
		_, diff := d.choose(difflen, xp, yp)
		switch diff {
		case Remove:
			edits, xs = append(edits, d.remove(xs[0])), xs[1:]
		case Insert:
			edits, ys = append(edits, d.insert(ys[0])), ys[1:]
		default: // keep
			edits, xs, ys = append(edits, d.keep(xs[0])), xs[1:], ys[1:]
		}
	}
}

func (d *differ[T]) insert(x T) Edit[T] {
	return Edit[T]{Insert, x}
}

func (d *differ[T]) remove(x T) Edit[T] {
	return Edit[T]{Remove, x}
}

func (d *differ[T]) keep(x T) Edit[T] {
	return Edit[T]{Keep, x}
}
