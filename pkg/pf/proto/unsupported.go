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

package proto

import (
	"context"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func (Provider) InternalValidate() error { panic("Unimplemented") }
func (Provider) Validate(ctx context.Context, c shim.ResourceConfig) ([]string, []error) {
	panic("Unimplemented")
}
func (Provider) ValidateResource(ctx context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	panic("Unimplemented")
}
func (Provider) ValidateDataSource(ctx context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	panic("Unimplemented")
}

func (Provider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	panic("Unimplemented")
}

func (Provider) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	panic("Unimplemented")
}

func (Provider) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("Unimplemented")
}

func (Provider) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	panic("Unimplemented")
}

func (Provider) ReadDataDiff(ctx context.Context, t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("Unimplemented")
}
func (Provider) ReadDataApply(ctx context.Context, t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("Unimplemented")
}

func (Provider) Meta(ctx context.Context) interface{} { panic("Unimplemented") }
func (Provider) Stop(ctx context.Context) error       { panic("Unimplemented") }

func (Provider) InitLogging(ctx context.Context) { panic("Unimplemented") }

// Create a Destroy diff for a resource identified by the TF token t.
func (Provider) NewDestroyDiff(ctx context.Context, t string, opts shim.TimeoutOptions) shim.InstanceDiff {
	panic("Unimplemented")
}

func (Provider) NewResourceConfig(ctx context.Context, object map[string]interface{}) shim.ResourceConfig {
	panic("Unimplemented")
}

func (Provider) NewProviderConfig(ctx context.Context, object map[string]interface{}) shim.ResourceConfig {
	panic("Unimplemented")
}

// Checks if a value is representing a Set, and unpacks its elements on success.
func (Provider) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	panic("Unimplemented")
}

func (Provider) SupportsUnknownCollections() bool {
	panic("Unimplemented")
}
