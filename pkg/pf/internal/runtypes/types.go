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

package runtypes

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type Schema interface {
	tftypes.AttributePathStepper

	Type(context.Context) tftypes.Type

	// Resource schemas are versioned for [State Upgrade].
	//
	// [State Upgrade]: https://developer.hashicorp.com/terraform/plugin/framework/resources/state-upgrade
	ResourceSchemaVersion() int64

	Shim() shim.SchemaMap
	DeprecationMessage() string

	ResourceProtoSchema(ctx context.Context) (*tfprotov6.Schema, error)

	TFName() TypeName
}

// Full resource type, including the provider type prefix and an underscore. For example,
// examplecloud_thing.
type TypeName string

// This could be a [TypeName] indicating a real Terraform type or else a TypeName+RenamedEntitySuffix pseudo-type introduced by [info.RenameResourceWithAlias].
type TypeOrRenamedEntityName string

type collection interface {
	All() []TypeOrRenamedEntityName
	Has(TypeOrRenamedEntityName) bool
	Schema(TypeOrRenamedEntityName) Schema
}

// Represents all provider's resources pre-indexed by TypeOrRenamedEntityName.
type Resources interface {
	collection
	IsResources()
}

// Represents all provider's datasources pre-indexed by TypeOrRenamedEntityName.
type DataSources interface {
	collection
	IsDataSources()
}
