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

type unionResourceMap struct {
	baseline  shim.ResourceMap
	extension shim.ResourceMap
}

var _ shim.ResourceMapWithClone = (*unionResourceMap)(nil)

func newUnionResourceMap(baseline, extension shim.ResourceMap) *unionResourceMap {
	return &unionResourceMap{
		baseline:  baseline,
		extension: extension,
	}
}

func (m *unionResourceMap) Len() int {
	n := 0
	m.baseline.Range(func(key string, value shim.Resource) bool {
		n++
		return true
	})
	m.extension.Range(func(key string, value shim.Resource) bool {
		if _, conflict := m.baseline.GetOk(key); !conflict {
			n++
		}
		return true
	})
	return n
}

func (m *unionResourceMap) Get(key string) shim.Resource {
	if v, ok := m.GetOk(key); ok {
		return v
	}
	contract.Failf("key not found: %v", key)
	return nil
}

func (m *unionResourceMap) GetOk(key string) (shim.Resource, bool) {
	if v, ok := m.baseline.GetOk(key); ok {
		return v, true
	}
	if v, ok := m.extension.GetOk(key); ok {
		return v, true
	}
	return nil, false
}

func (m *unionResourceMap) Range(each func(key string, value shim.Resource) bool) {
	iterating := true
	m.baseline.Range(func(key string, value shim.Resource) bool {
		if value == nil {
			panic("GAH1! nil resource in baseline")
		}
		iterating = iterating && each(key, value)
		return iterating
	})
	if !iterating {
		return
	}
	m.extension.Range(func(key string, value shim.Resource) bool {
		if _, conflict := m.baseline.GetOk(key); conflict {
			return true
		}
		if value == nil {
			panic("GAH2! nil resource in extension")
		}
		return each(key, value)
	})
}

func (m *unionResourceMap) Set(key string, value shim.Resource) {
	if value == nil {
		panic("GAH! nil in Set")
	}
	// Sending edits to the owner map.
	_, b := m.baseline.GetOk(key)
	_, e := m.extension.GetOk(key)
	switch {
	case b && e:
		// In the case of conflict, send the edit to both.
		m.baseline.Set(key, value)
		m.extension.Set(key, value)
	case b:
		m.baseline.Set(key, value)
	case e:
		m.extension.Set(key, value)
	default:
		// Net-new keys accumulate in baseline, this is arbitrary:
		m.baseline.Set(key, value)
	}
}

func (m *unionResourceMap) Clone(oldKey, newKey string) error {
	// Sending the clone operation to the owner map.
	_, b := m.baseline.GetOk(oldKey)
	_, e := m.extension.GetOk(oldKey)

	switch {
	case b && e:
		// In the case of conflict, send the clone operation to both.
		m.baseline.Set(newKey, m.baseline.Get(oldKey))
		m.extension.Set(newKey, m.extension.Get(oldKey))
		return nil
	case b:
		m.baseline.Set(newKey, m.baseline.Get(oldKey))
		return nil
	case e:
		m.extension.Set(newKey, m.extension.Get(oldKey))
		return nil
	default:
		return fmt.Errorf("Cannot clone non-existing key %q to %q", oldKey, newKey)
	}
}

func (m *unionResourceMap) ConflictingKeys() []string {
	conflicts := []string{}
	m.baseline.Range(func(key string, value shim.Resource) bool {
		if _, ok := m.extension.GetOk(key); ok {
			conflicts = append(conflicts, key)
		}
		return true
	})
	sort.Strings(conflicts)
	return nil
}
