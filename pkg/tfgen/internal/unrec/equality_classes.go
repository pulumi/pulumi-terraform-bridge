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

package unrec

func equalityClasses[T any](eq func(T, T) bool, slice []T) [][]T {
	switch len(slice) {
	case 0:
		return nil
	case 1:
		return [][]T{slice}
	case 2:
		a, b := slice[0], slice[1]
		if eq(a, b) {
			return [][]T{slice}
		}
		return [][]T{{a}, {b}}
	default:
		n := len(slice)
		left := slice[0 : n/2]
		right := slice[n/2:]
		part1 := equalityClasses(eq, left)
		part2 := equalityClasses(eq, right)
		return mergeEqualityClasses(eq, part1, part2)
	}
}

func sameEqualityClasses[T any](eq func(T, T) bool, xs []T, ys []T) bool {
	if len(xs) == 0 || len(ys) == 0 {
		return len(xs) == len(ys)
	}
	return eq(xs[0], ys[0])
}

func mergeEqualityClasses[T any](eq func(T, T) bool, xs [][]T, ys [][]T) (result [][]T) {
	acc := [][]T{}
	for _, xc := range xs {
		if len(xc) == 0 {
			continue
		}
		acc = append(acc, xc)
	}
	for _, yc := range ys {
		matched := false
		for i, ac := range acc {
			if sameEqualityClasses(eq, ac, yc) {
				acc[i] = append(append([]T{}, ac...), yc...)
				matched = true
				break
			}
		}
		if !matched {
			acc = append(acc, yc)
		}
	}
	return acc
}
