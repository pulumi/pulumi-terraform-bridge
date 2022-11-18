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

package testprovider

type PortModel struct {
	Handlers []string `tfsdk:"handlers" json:"handlers"`
	Port     *int64   `tfsdk:"port" json:"port"`
}

type ServiceModel struct {
	InternalPort *int64      `tfsdk:"intport" json:"intport"`
	Ports        []PortModel `tfsdk:"ports" json:"ports"`
	Protocol     *string     `tfsdk:"protocol" json:"protocol"`
}

type SingleNestedAttrModel struct {
	Description *string  `tfsdk:"description" json:"description"`
	Quantity    *float64 `tfsdk:"quantity" json:"quantity"`
}
