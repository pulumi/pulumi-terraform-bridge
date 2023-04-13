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

package muxer

// Serializable dispatch table maps TF resource, function and config tokens to numbers 0..(N-1) identifying which of N
// muxed providers defines the resource, function or config value.
//
// Keys are tokens from Pulumi Package schema, or in case of Config, Pulumi property names.
type DispatchTable struct {
	// Resources and functions can only map to a single provider
	Resources        map[string]int `json:"resources"`
	ResourcesDefault *int           `json:"resourcesDefault"`
	Functions        map[string]int `json:"functions"`
	FunctionsDefault *int           `json:"functionsDefault"`

	// Config values can map to multiple providers
	Config        map[string][]int `json:"config"`
	ConfigDefault []int            `json:"configDefault"`
}

func NewDispatchTable() *DispatchTable {
	return &DispatchTable{
		Resources: make(map[string]int),
		Functions: make(map[string]int),
		Config:    make(map[string][]int),
	}
}

func (m *DispatchTable) DispatchFunction(token string) (int, bool) {
	i, ok := m.Functions[token]
	if ok {
		return i, true
	}
	if m.FunctionsDefault != nil {
		return *m.FunctionsDefault, true
	}
	return 0, false
}

func (m *DispatchTable) DispatchResource(token string) (int, bool) {
	i, ok := m.Resources[token]
	if ok {
		return i, true
	}
	if m.ResourcesDefault != nil {
		return *m.ResourcesDefault, true
	}
	return 0, false
}

func (m *DispatchTable) DispatchConfig(key string) ([]int, bool) {
	i, ok := m.Config[key]
	if ok {
		return i, true
	}
	if m.ConfigDefault != nil {
		return m.ConfigDefault, true
	}
	return nil, false
}
