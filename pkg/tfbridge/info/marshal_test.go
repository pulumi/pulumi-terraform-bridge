package info

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/stretchr/testify/require"
)

func TestSchema(t *testing.T) {
	sch := schema.Schema{
		Type:    schema.TypeString,
		Default: "default",
	}
	shimSchema := sdkv2.NewSchema(&sch)

	marshalled := MarshalSchemaShim(shimSchema)
	unmarshalled := marshalled.Unmarshal()

	require.Equal(t, shim.ValueType(sch.Type), unmarshalled.Type())
}
