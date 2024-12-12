package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

type tfResourceName string

func translateTypeName(bridgedProvider *info.Provider, n tfResourceName) string {
	for r, rr := range bridgedProvider.Resources {
		if r == string(n) {
			return string(rr.Tok)
		}
	}
	panic(fmt.Sprintf("Unknown type name: %s", n))
}

var resourceNameCounter atomic.Int32

func translateResourceName(plannedState *tfprotov6.DynamicValue) string {
	// The provider does not have access to the resource name surprisingly (!).
	n := resourceNameCounter.Add(1)
	return fmt.Sprintf("r%d", n)
}

func translateResourceArgs(
	ctx context.Context,
	n tfResourceName,
	dv *tfprotov6.DynamicValue,
	resourceSchemas map[string]*tfprotov6.Schema,
	bridgedProvider *info.Provider,
) (*structpb.Struct, error) {
	rschema, ok := resourceSchemas[string(n)]
	if !ok {
		return nil, fmt.Errorf("Unknown resource: %q", n)
	}
	objectType, ok := rschema.ValueType().(tftypes.Object)
	if !ok {
		return nil, fmt.Errorf("Bad object type for resource: %q", n)
	}
	encoding := convert.NewEncoding(bridgedProvider.P, bridgedProvider)
	dec, err := encoding.NewResourceDecoder(string(n), objectType)
	if err != nil {
		return nil, fmt.Errorf("Failed to derive a resource decoder: %v", err)
	}
	pm, err := convert.DecodePropertyMapFromDynamic(ctx, dec, objectType, dv)
	if err != nil {
		return nil, err
	}
	return plugin.MarshalProperties(pm, plugin.MarshalOptions{
		Label:            "translateResourceArgs",
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
}

func translateResourceOutputs(
	n tfResourceName,
	outputs *structpb.Struct,
	resourceSchemas map[string]*tfprotov6.Schema,
	bridgedProvider *info.Provider,
) (*tfprotov6.DynamicValue, error) {
	propMap, err := plugin.UnmarshalProperties(outputs, plugin.MarshalOptions{
		Label:            "translateResourceOutputs",
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	if err != nil {
		return nil, err
	}

	rschema, ok := resourceSchemas[string(n)]
	if !ok {
		return nil, fmt.Errorf("Unknown resource: %q", n)
	}
	objectType, ok := rschema.ValueType().(tftypes.Object)
	if !ok {
		return nil, fmt.Errorf("Bad object type for resource: %q", n)
	}
	encoding := convert.NewEncoding(bridgedProvider.P, bridgedProvider)
	enc, err := encoding.NewResourceEncoder(string(n), objectType)
	if err != nil {
		return nil, fmt.Errorf("Failed to derive a resource encoder: %v", err)
	}
	return convert.EncodePropertyMapToDynamic(enc, objectType, propMap)
}
