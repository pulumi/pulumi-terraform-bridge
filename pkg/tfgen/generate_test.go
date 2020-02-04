package tfgen

import (
	"fmt"
	"math"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
	"github.com/stretchr/testify/assert"
)

type defaultValueTest struct {
	schema       schema.Schema
	info         tfbridge.DefaultInfo
	expectedNode string
}

var defaultTests = []defaultValueTest{
	{
		schema:       schema.Schema{Type: schema.TypeString},
		expectedNode: "undefined",
	},
	{
		schema:       schema.Schema{Type: schema.TypeBool},
		info:         tfbridge.DefaultInfo{Value: false},
		expectedNode: "false",
	},
	{
		schema:       schema.Schema{Type: schema.TypeBool},
		info:         tfbridge.DefaultInfo{Value: true},
		expectedNode: "true",
	},
	{
		schema:       schema.Schema{Type: schema.TypeInt},
		info:         tfbridge.DefaultInfo{Value: 0x4eedface},
		expectedNode: "1324219086",
	},
	{
		schema:       schema.Schema{Type: schema.TypeInt},
		info:         tfbridge.DefaultInfo{Value: uint(0xfeedface)},
		expectedNode: "4277009102",
	},
	{
		schema:       schema.Schema{Type: schema.TypeFloat},
		info:         tfbridge.DefaultInfo{Value: math.Pi},
		expectedNode: "3.141592653589793",
	},
	{
		schema:       schema.Schema{Type: schema.TypeString},
		info:         tfbridge.DefaultInfo{Value: "foo"},
		expectedNode: `"foo"`,
	},
}

func Test_NodeDefaults(t *testing.T) {
	for _, dvt := range defaultTests {
		v := &variable{
			name:   "v",
			schema: &dvt.schema,
			info:   &tfbridge.SchemaInfo{Default: &dvt.info},
		}

		actual := tsDefaultValue(v)
		assert.Equal(t, dvt.expectedNode, actual)

		getType := ""
		switch dvt.schema.Type {
		case schema.TypeBool:
			getType = "Boolean"
		case schema.TypeInt, schema.TypeFloat:
			getType = "Number"
		}

		singleEnv := fmt.Sprintf(`utilities.getEnv%s("FOO")`, getType)
		multiEnv := fmt.Sprintf(`utilities.getEnv%s("FOO", "BAR")`, getType)
		if dvt.expectedNode != "undefined" {
			singleEnv = fmt.Sprintf("(%s || %s)", singleEnv, dvt.expectedNode)
			multiEnv = fmt.Sprintf("(%s || %s)", multiEnv, dvt.expectedNode)
		}

		v.info.Default.EnvVars = []string{"FOO"}
		actual = tsDefaultValue(v)
		assert.Equal(t, singleEnv, actual)

		v.info.Default.EnvVars = []string{"FOO", "BAR"}
		actual = tsDefaultValue(v)
		assert.Equal(t, multiEnv, actual)
	}
}

func Test_DeprecationFromTFSchema(t *testing.T) {
	v := &variable{
		name:   "v",
		schema: &schema.Schema{Type: schema.TypeString, Deprecated: "This is deprecated"},
	}

	deprecationMessage := v.deprecationMessage()
	assert.Equal(t, "This is deprecated", deprecationMessage)
}
