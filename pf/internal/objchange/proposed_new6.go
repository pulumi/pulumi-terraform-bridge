package objchange

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/pulumi/terraform/pkg/configs/configschema"
	"github.com/pulumi/terraform/pkg/plans/objchange"
	"github.com/pulumi/terraform/pkg/plugin6/convert"
	"github.com/pulumi/terraform/pkg/tfplugin6"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/objchange/toproto"
)

// Exposes ProposedNew algo in terms of terraform-plugin-go types.
func ProposedNew6(ctx context.Context, schema *tfprotov6.SchemaBlock, priorState,
	config tftypes.Value) (tftypes.Value, error) {

	typ := priorState.Type()
	nothing := tftypes.NewValue(typ, nil)

	schemaBlock, err := convertSchemaBlock(schema)
	if err != nil {
		return nothing, fmt.Errorf("ProposedNew6 failed to convert schema: %w", err)
	}

	ty := schemaBlock.ImpliedType()

	priorStateCtyValue, err := toCtyValue6(ty, typ, priorState)
	if err != nil {
		return nothing, fmt.Errorf("ProposedNew6 failed to convert priorState: %w", err)
	}

	configCtyValue, err := toCtyValue6(ty, typ, config)
	if err != nil {
		return nothing, fmt.Errorf("ProposedNew6 failed to convert config: %w", err)
	}

	v := objchange.ProposedNew(schemaBlock, priorStateCtyValue, configCtyValue)

	proposed, err := fromCtyValue6(ty, typ, v)
	if err != nil {
		return nothing, fmt.Errorf("ProposedNew6 failed to convert back the result: %w", err)
	}

	return proposed, nil
}

func convertSchemaBlock(schema *tfprotov6.SchemaBlock) (*configschema.Block, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}
	rawProtoV6SchemaBlock, err := toProto(schema)
	if err != nil {
		return nil, err
	}
	var protoSchemaBlock tfplugin6.Schema_Block
	if err := protobuf.Unmarshal(rawProtoV6SchemaBlock, &protoSchemaBlock); err != nil {
		return nil, err
	}
	return convert.ProtoToConfigSchema(&protoSchemaBlock), nil
}

func fromCtyValue6(ty cty.Type, typ tftypes.Type, v cty.Value) (tftypes.Value, error) {
	msgPack, err := msgpack.Marshal(v, ty)
	if err != nil {
		return tftypes.NewValue(typ, nil), err
	}
	dv := tfprotov6.DynamicValue{MsgPack: msgPack}
	return dv.Unmarshal(typ)
}

func toCtyValue6(ty cty.Type, typ tftypes.Type, v tftypes.Value) (cty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(typ, v)
	if err != nil {
		return cty.NullVal(ty), err
	}
	return msgpack.Unmarshal(dv.MsgPack, ty)
}

func toProto(schema *tfprotov6.SchemaBlock) ([]byte, error) {
	protoSchema, err := toproto.Schema_Block(schema)
	if err != nil {
		return nil, err
	}
	return protobuf.Marshal(protoSchema)
}
