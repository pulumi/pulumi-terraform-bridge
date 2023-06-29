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

package x

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// A generic remapping strategy.
type Strategy[T tfbridge.ResourceInfo | tfbridge.DataSourceInfo] func(tfToken string) (*T, error)

// Describe the mapping from resource and datasource tokens to Pulumi resources and
// datasources.
type DefaultStrategy struct {
	Resource   ResourceStrategy
	DataSource DataSourceStrategy
}

// A strategy for generating missing resources.
type ResourceStrategy = Strategy[tfbridge.ResourceInfo]

// A strategy for generating missing datasources.
type DataSourceStrategy = Strategy[tfbridge.DataSourceInfo]
