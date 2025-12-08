package adapter

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

// From the bridge
// TODO: add a new interface for InstanceState for MakeTerraformResult
type adapterInstanceState struct {
	resourceType string
	stateValue   cty.Value
	// TODO: meta handling
	meta map[string]interface{}
}

var (
	_ shim.InstanceState               = adapterInstanceState{}
	_ shim.InstanceStateWithTypedValue = adapterInstanceState{}
)

func (s adapterInstanceState) ID() string {
	if s.stateValue.IsNull() {
		return ""
	}
	id := s.stateValue.GetAttr("id")
	if !id.IsKnown() {
		return ""
	}
	contract.Assertf(id.Type() == cty.String, "expected id to be of type String")
	return id.AsString()
}

func (s adapterInstanceState) Meta() map[string]interface{} {
	return s.meta
}

func (s adapterInstanceState) Value() valueshim.Value {
	return valueshim.FromCtyValue(s.stateValue)
}

func (s adapterInstanceState) Object(sch shim.SchemaMap) (map[string]interface{}, error) {
	res := objectFromCtyValue(s.stateValue)
	// grpc servers add a "timeouts" key to compensate for infinite diffs; this is not needed in
	// the Pulumi projection.
	delete(res, schema.TimeoutsConfigKey)
	return res, nil
}

func (s adapterInstanceState) Type() string {
	return s.resourceType
}

type setChecker struct{}

func (s setChecker) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	return nil, false
}

func convertTFValueToPulumiValue(
	tfValue cty.Value, resourceType string, res shim.Resource, pulumiResource *info.Resource,
) (resource.PropertyMap, error) {
	instanceState := adapterInstanceState{
		resourceType: resourceType,
		stateValue:   tfValue,
		// TODO: meta handling
		meta: nil,
	}

	// TODO: schema upgrades - what if the schema version is different?
	props, err := tfbridge.MakeTerraformResult(context.TODO(), setChecker{}, instanceState, res.Schema(), pulumiResource.Fields, nil, true)

	// TODO: fix raw states
	// if err := tfbridge.RawStateInjectDelta(context.TODO(), res.Schema(), pulumiResource.Fields, props, res.SchemaType(), instanceState); err != nil {
	// 	return nil, err
	// }
	return props, err
}
