package schemashim

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestMapAttribute(t *testing.T) {
	t.Parallel()
	mapAttr := schema.MapAttribute{
		Optional:    true,
		ElementType: basetypes.StringType{},
	}
	shimmed := &typeSchema{mapAttr.GetType(), nil, internalinter.Internal{}}
	assertIsMapType(t, shimmed)
	s := shimmed.Elem().(*typeSchema)
	assert.Equal(t, shim.TypeString, s.Type())
}

func assertIsMapType(t *testing.T, shimmed shim.Schema) {
	assert.Equal(t, shim.TypeMap, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	schema, isTypeSchema := shimmed.Elem().(*typeSchema)
	assert.Truef(t, isTypeSchema, "expected shim.Elem() to be of type %T", *schema)
}

func TestInt32Type(t *testing.T) {
	t.Parallel()
	shimmed := &typeSchema{basetypes.Int32Type{}, nil, internalinter.Internal{}}
	assert.Equal(t, shim.TypeInt, shimmed.Type())
}

func TestInt64Type(t *testing.T) {
	t.Parallel()
	shimmed := &typeSchema{basetypes.Int64Type{}, nil, internalinter.Internal{}}
	assert.Equal(t, shim.TypeInt, shimmed.Type())
}

func TestNumberType(t *testing.T) {
	t.Parallel()
	shimmed := &typeSchema{basetypes.NumberType{}, nil, internalinter.Internal{}}
	assert.Equal(t, shim.TypeFloat, shimmed.Type())
}
