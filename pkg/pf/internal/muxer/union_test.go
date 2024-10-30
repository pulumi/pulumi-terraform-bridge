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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnionMap(t *testing.T) {
	m1 := testMap{
		"one": "n1",
		"two": "n2",
	}
	m2 := testMap{
		"two":   "x2",
		"three": "x3",
	}
	m := newUnionMap(m1, m2)

	t.Run("Len", func(t *testing.T) {
		assert.Equal(t, 3, m.Len())
	})
	t.Run("GetOk", func(t *testing.T) {
		_, ok := m.GetOk("zero")
		assert.False(t, ok)
		n1, ok := m.GetOk("one")
		assert.Equal(t, "n1", n1)
		assert.True(t, ok)
		// Conflicting keys served from the left (baseline) map.
		n2, ok := m.GetOk("two")
		assert.Equal(t, "n2", n2)
		assert.True(t, ok)
		n3, ok := m.GetOk("three")
		assert.Equal(t, "x3", n3)
		assert.True(t, ok)
	})
	t.Run("Get", func(t *testing.T) {
		n1 := m.Get("one")
		assert.Equal(t, "n1", n1)
	})
	t.Run("ConflictingKeys", func(t *testing.T) {
		assert.Equal(t, []string{"two"}, m.ConflictingKeys())
	})
	t.Run("Range", func(t *testing.T) {
		keys := []string{}
		values := []string{}
		m.Range(func(key, value string) bool {
			keys = append(keys, key)
			values = append(values, value)
			return true
		})
		sort.Strings(keys)
		sort.Strings(values)
		assert.Equal(t, []string{"one", "three", "two"}, keys)
		assert.Equal(t, []string{"n1", "n2", "x3"}, values)
	})
	t.Run("Set", func(t *testing.T) {
		m1 := testMap{
			"one": "n1",
			"two": "n2",
		}
		m2 := testMap{
			"two":   "x2",
			"three": "x3",
		}
		m := newUnionMap(m1, m2)
		m.Set("one", "n1!")
		assert.Equal(t, "n1!", m1["one"])
		m.Set("three", "x3!")
		assert.Equal(t, "x3!", m2["three"])
		m.Set("zero", "n0!")
		assert.Equal(t, "n0!", m1["zero"])
		t.Run("conflict", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("The code did not panic")
				}
			}()
			m.Set("two", "n2!")
		})
	})
	t.Run("Clone", func(t *testing.T) {
		m1 := testMap{
			"one": "n1",
			"two": "n2",
		}
		m2 := testMap{
			"two":   "x2",
			"three": "x3",
		}
		m := newUnionMap(m1, m2)
		err := m.Clone("one", "one_legacy")
		assert.NoError(t, err)
		assert.Equal(t, "n1", m1["one_legacy"])
		err = m.Clone("three", "three_legacy")
		assert.NoError(t, err)
		assert.Equal(t, "x3", m2["three_legacy"])
		assert.Error(t, m.Clone("two", "two_legacy"))
		assert.Error(t, m.Clone("zero", "zero_legacy"))
	})
}

type testMap map[string]string

var _ mapLike[string] = make(testMap)

func (x testMap) Len() int {
	return len(x)
}

func (x testMap) GetOk(key string) (string, bool) {
	v, ok := x[key]
	return v, ok
}

func (x testMap) Set(key string, value string) {
	x[key] = value
}

func (x testMap) Range(each func(key string, value string) bool) {
	for k, v := range x {
		if !each(k, v) {
			return
		}
	}
}
