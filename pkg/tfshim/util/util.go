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

package util

import (
	"context"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = (shim.Provider)((*UnimplementedProvider)(nil))

// An embed-able unimplemented Provider for use in testing.
type UnimplementedProvider struct{}

func (UnimplementedProvider) Schema() shim.SchemaMap           { panic("unimplemented") }
func (UnimplementedProvider) ResourcesMap() shim.ResourceMap   { panic("unimplemented") }
func (UnimplementedProvider) DataSourcesMap() shim.ResourceMap { panic("unimplemented") }

func (UnimplementedProvider) Validate(
	ctx context.Context, c shim.ResourceConfig,
) ([]string, []error) {
	panic("unimplemented")
}

func (UnimplementedProvider) ValidateResource(
	ctx context.Context, t string, c shim.ResourceConfig,
) ([]string, []error) {
	panic("unimplemented")
}

func (UnimplementedProvider) ValidateDataSource(
	ctx context.Context, t string, c shim.ResourceConfig,
) ([]string, []error) {
	panic("unimplemented")
}

func (UnimplementedProvider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	panic("unimplemented")
}

func (UnimplementedProvider) Diff(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig, opts ...shim.DiffOption,
) (shim.InstanceDiff, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) ReadDataDiff(
	ctx context.Context, t string, c shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) ReadDataApply(
	ctx context.Context, t string, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) Meta(ctx context.Context) interface{} { panic("unimplemented") }
func (UnimplementedProvider) Stop(ctx context.Context) error       { panic("unimplemented") }

func (UnimplementedProvider) InitLogging(ctx context.Context) { panic("unimplemented") }

func (UnimplementedProvider) NewDestroyDiff(ctx context.Context, t string) shim.InstanceDiff {
	panic("unimplemented")
}

func (UnimplementedProvider) NewResourceConfig(
	ctx context.Context, object map[string]interface{},
) shim.ResourceConfig {
	panic("unimplemented")
}

func (UnimplementedProvider) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	panic("unimplemented")
}
