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

type matrix struct {
	m    int
	n    int
	data []int
}

func newMatrix(m, n int) *matrix {
	data := make([]int, m*n)
	return &matrix{m, n, data}
}

func (m *matrix) get(i, j int) int {
	return m.data[m.index(i, j)]
}

func (m *matrix) set(i, j int, v int) {
	m.data[m.index(i, j)] = v
}

func (m *matrix) index(i, j int) int {
	return m.n*i + j
}
