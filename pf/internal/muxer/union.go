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

package muxer

import (
	"fmt"
	"sort"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type unionMap[T any] struct {
	baseline  mapLike[T]
	extension mapLike[T]
}

type mapLike[T any] interface {
	Len() int
	Range(func(key string, value T) bool)
	GetOk(key string) (T, bool)
	Set(key string, value T)
}

var _ shim.ResourceMapWithClone = (*unionMap[shim.Resource])(nil)

func newUnionMap[T any](baseline, extension mapLike[T]) *unionMap[T] {
	return &unionMap[T]{
		baseline:  baseline,
		extension: extension,
	}
}

func (m *unionMap[T]) Len() int {
	n := m.baseline.Len()
	m.extension.Range(func(key string, value T) bool {
		if _, conflict := m.baseline.GetOk(key); !conflict {
			n++
		}
		return true
	})
	return n
}

func (m *unionMap[T]) Get(key string) T {
	if v, ok := m.GetOk(key); ok {
		return v
	}
	contract.Failf("key not found: %v", key)
	var zero T
	return zero
}

func (m *unionMap[T]) GetOk(key string) (T, bool) {
	if v, ok := m.baseline.GetOk(key); ok {
		return v, true
	}
	if v, ok := m.extension.GetOk(key); ok {
		return v, true
	}
	var zero T
	return zero, false
}

func (m *unionMap[T]) Range(each func(key string, value T) bool) {
	iterating := true
	m.baseline.Range(func(key string, value T) bool {
		iterating = iterating && each(key, value)
		return iterating
	})
	if !iterating {
		return
	}
	m.extension.Range(func(key string, value T) bool {
		if _, conflict := m.baseline.GetOk(key); conflict {
			return true
		}
		return each(key, value)
	})
}

func (m *unionMap[T]) Set(key string, value T) {
	// Sending edits to the owner map.
	_, b := m.baseline.GetOk(key)
	_, e := m.extension.GetOk(key)
	switch {
	case b && e:
		contract.Failf("Cannot set a conflicting key in a union of two maps: %q", key)
	case b:
		m.baseline.Set(key, value)
	case e:
		m.extension.Set(key, value)
	default:
		// Net-new keys accumulate in baseline, this is arbitrary:
		m.baseline.Set(key, value)
	}
}

// Clone will delegate the operation to the sub-map owning oldKey, or fail if the owner is ambiguous. In the sub-map the
// value associated with oldKey will now be also associated with newKey. In Pulumi this is relied on to track ownership
// of aliased resources in muxed providers as RenameResourceWithAlias uses Clone.
func (m *unionMap[T]) Clone(oldKey, newKey string) error {
	// Sending the clone operation to the owner map.
	bv, b := m.baseline.GetOk(oldKey)
	ev, e := m.extension.GetOk(oldKey)

	switch {
	case b && e:
		return fmt.Errorf("Cannot clone a conflicting key in a union of two maps: %q", oldKey)
	case b:
		m.baseline.Set(newKey, bv)
		return nil
	case e:
		m.extension.Set(newKey, ev)
		return nil
	default:
		return fmt.Errorf("Cannot clone a non-existing key %q to %q", oldKey, newKey)
	}
}

func (m *unionMap[T]) ConflictingKeys() []string {
	conflicts := []string{}
	m.baseline.Range(func(key string, value T) bool {
		if _, ok := m.extension.GetOk(key); ok {
			conflicts = append(conflicts, key)
		}
		return true
	})
	sort.Strings(conflicts)
	return conflicts
}
