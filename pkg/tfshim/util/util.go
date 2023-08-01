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
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = (shim.Provider)((*UnimplementedProvider)(nil))

// An embed-able unimplemented Provider for use in testing.
type UnimplementedProvider struct{}

func (UnimplementedProvider) Schema() shim.SchemaMap           { panic("unimplemented") }
func (UnimplementedProvider) ResourcesMap() shim.ResourceMap   { panic("unimplemented") }
func (UnimplementedProvider) DataSourcesMap() shim.ResourceMap { panic("unimplemented") }

func (UnimplementedProvider) Validate(c shim.ResourceConfig) ([]string, []error) {
	panic("unimplemented")
}
func (UnimplementedProvider) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("unimplemented")
}
func (UnimplementedProvider) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("unimplemented")
}

func (UnimplementedProvider) Configure(c shim.ResourceConfig) error { panic("unimplemented") }
func (UnimplementedProvider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("unimplemented")
}
func (UnimplementedProvider) Apply(t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("unimplemented")
}
func (UnimplementedProvider) Refresh(string, shim.InstanceState, shim.ResourceConfig) (shim.InstanceState, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) ReadDataDiff(t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("unimplemented")
}
func (UnimplementedProvider) ReadDataApply(t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("unimplemented")
}

func (UnimplementedProvider) Meta() interface{} { panic("unimplemented") }
func (UnimplementedProvider) Stop() error       { panic("unimplemented") }

func (UnimplementedProvider) InitLogging()                      { panic("unimplemented") }
func (UnimplementedProvider) NewDestroyDiff() shim.InstanceDiff { panic("unimplemented") }
func (UnimplementedProvider) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	panic("unimplemented")
}
func (UnimplementedProvider) IsSet(v interface{}) ([]interface{}, bool) { panic("unimplemented") }
