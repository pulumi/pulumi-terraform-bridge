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

package tfbridge

// This file exists to provide backwards compatibility.
//
// To add new functionality here, add to ./tokens

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

// Describe the mapping from TF resource and datasource tokens to Pulumi resources and
// datasources.
type Strategy = tokens.Strategy

// A strategy for generating missing resources.
type ResourceStrategy = info.ElementStrategy[ResourceInfo]

// A strategy for generating missing datasources.
type DataSourceStrategy = info.ElementStrategy[DataSourceInfo]
