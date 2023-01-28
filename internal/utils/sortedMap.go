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

package utils

import (
	"sort"

	"golang.org/x/exp/constraints"
)

// An interface constaint for comparable and ordered
type OrderdKey interface {
	constraints.Ordered
	comparable
}

// A map that yields its keys in sorted order.
type SortedMap[K OrderdKey, V any] struct {
	keys []K
	m    map[K]V
}

// Create a sorted map from a normal map.
func NewSortedMap[K OrderdKey, V any](m map[K]V) *SortedMap[K, V] {
	return &SortedMap[K, V]{
		m: m,
	}
}

func (sm *SortedMap[K, V]) Keys() []K {
	if sm == nil {
		return nil
	}
	if sm.keys == nil {
		sm.keys = make([]K, 0, len(sm.m))
		for k := range sm.m {
			sm.keys = append(sm.keys, k)
		}
		sort.Slice(sm.keys, func(i, j int) bool {
			return sm.keys[i] < sm.keys[j]
		})
	}
	return sm.keys
}

func (sm *SortedMap[K, V]) Get(key K) (V, bool) {
	if sm == nil || sm.m == nil {
		return Zero[V](), false
	}
	v, ok := sm.m[key]
	return v, ok
}

func Zero[T any]() T {
	var t T
	return t
}
