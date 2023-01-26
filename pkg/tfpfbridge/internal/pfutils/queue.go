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

// Remove and return the value at a random key.
func pop[T any](q map[string]T) (string, T) {
	for k := range q {
		return k, popAt(q, k)
	}
	panic("empty queue")
}

// Remove and return the value at key.
func popAt[T any](q map[string]T, key string) T {
	if v, ok := q[key]; ok {
		delete(q, key)
		return v
	}
	panic("key no found: " + key)
}
