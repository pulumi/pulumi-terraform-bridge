package proto

import (
	"context"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func (SchemaOnlyProvider) InternalValidate() error { panic("Unimplemented") }
func (SchemaOnlyProvider) Validate(ctx context.Context, c shim.ResourceConfig) ([]string, []error) {
	panic("Unimplemented")
}
func (SchemaOnlyProvider) ValidateResource(ctx context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	panic("Unimplemented")
}
func (SchemaOnlyProvider) ValidateDataSource(ctx context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) Apply(ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) ReadDataDiff(ctx context.Context, t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("Unimplemented")
}
func (SchemaOnlyProvider) ReadDataApply(ctx context.Context, t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) Meta(ctx context.Context) interface{} { panic("Unimplemented") }
func (SchemaOnlyProvider) Stop(ctx context.Context) error       { panic("Unimplemented") }

func (SchemaOnlyProvider) InitLogging(ctx context.Context) { panic("Unimplemented") }

// Create a Destroy diff for a resource identified by the TF token t.
func (SchemaOnlyProvider) NewDestroyDiff(ctx context.Context, t string, opts shim.TimeoutOptions) shim.InstanceDiff {
	panic("Unimplemented")
}

func (SchemaOnlyProvider) NewResourceConfig(ctx context.Context, object map[string]interface{}) shim.ResourceConfig {
	panic("Unimplemented")
}

// Checks if a value is representing a Set, and unpacks its elements on success.
func (SchemaOnlyProvider) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	panic("Unimplemented")
}
