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

package provider

import (
	"context"
	"sync"

	otshim "github.com/opentofu/opentofu/shim"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ shim.Provider = (*shimProvider)(nil)

func New(p otshim.Provider) shim.Provider {
	return &shimProvider{p, sync.OnceValue(p.GetProviderSchema)}
}

type shimProvider struct {
	remote otshim.Provider

	schema func() otshim.ProviderSchema
}

// Unlike the GetProviderSchema on remote, Schema is just the schema of the provider
// itself (not associated resources or datasources).
func (p *shimProvider) Schema() shim.SchemaMap {
	schema := p.schema().Provider
	return object{schema}
}

func (p *shimProvider) ResourcesMap() shim.ResourceMap {
	panic("TODO")
}

func (p *shimProvider) DataSourcesMap() shim.ResourceMap {
	panic("TODO")
}

// Doesn't apply
func (p *shimProvider) InternalValidate() error { return nil }

func (p *shimProvider) Validate(ctx context.Context, c shim.ResourceConfig) ([]string, []error) {
	panic("Needs to be implemented in terms of p.remote.ValidateProviderConfig")
}

func (p *shimProvider) ValidateResource(ctx context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	panic("Needs to be implement in terms of p.remote.ValidateResourceConfig")
}

func (p *shimProvider) ValidateDataSource(ctx context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	panic("Needs to be implement in terms of p.remote.ValidateDataResourceConfig")
}

func (p *shimProvider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	panic("Needs to be implement in terms of p.remote.ConfigureProvider")
}

func (p *shimProvider) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	panic("Needs to be implement in terms of p.remote.PlanResourceChange")
}

func (p *shimProvider) Apply(ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("Needs to be implement in terms of p.remote.ApplyResourceChange")
}

func (p *shimProvider) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	panic("Needs to be implement in terms of p.remote.ReadResource")
}

func (p *shimProvider) ReadDataDiff(ctx context.Context, t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("Needs to be implement in terms of p.remote.ReadDataSource")
}

func (p *shimProvider) ReadDataApply(ctx context.Context, t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("I'm not sure what this does")
}

// Meta is impossible to implement, since it allows non-serializable values.
//
// That said, as long as we don't use the result of Meta it should be fine.
func (p *shimProvider) Meta(ctx context.Context) interface{} { return nil }

func (p *shimProvider) Stop(ctx context.Context) error { return p.remote.Stop() }

func (p *shimProvider) InitLogging(ctx context.Context) { /* no-op */ }

// Create a Destroy diff for a resource identified by the TF token t.
func (p *shimProvider) NewDestroyDiff(ctx context.Context, t string, opts shim.TimeoutOptions) shim.InstanceDiff {
	panic("Needs to be implement in terms of p.remote.PlanResourceChange")
}

func (p *shimProvider) NewResourceConfig(ctx context.Context, object map[string]interface{}) shim.ResourceConfig {
	panic("Needs to be implement in terms of p.remote.ApplyResourceChange, I think?")
}

// Checks if a value is representing a Set, and unpacks its elements on success.
func (p *shimProvider) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	panic("I'm not sure how or why this exists")
}
