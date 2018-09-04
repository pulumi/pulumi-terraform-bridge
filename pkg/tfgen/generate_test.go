package tfgen

import (
	"fmt"
	"math"
	"testing"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/pulumi/pulumi-terraform/pkg/tfbridge"
	"github.com/stretchr/testify/assert"
)

type defaultValueTest struct {
	schema         schema.Schema
	info           tfbridge.DefaultInfo
	expectedNode   string
	expectedGo     string
	expectedPython string
}

var defaultTests = []defaultValueTest{
	{
		schema:         schema.Schema{Type: schema.TypeString},
		expectedNode:   "undefined",
		expectedGo:     "",
		expectedPython: "",
	},
	{
		schema:         schema.Schema{Type: schema.TypeBool},
		info:           tfbridge.DefaultInfo{Value: false},
		expectedNode:   "false",
		expectedGo:     "false",
		expectedPython: "False",
	},
	{
		schema:         schema.Schema{Type: schema.TypeBool},
		info:           tfbridge.DefaultInfo{Value: true},
		expectedNode:   "true",
		expectedGo:     "true",
		expectedPython: "True",
	},
	{
		schema:         schema.Schema{Type: schema.TypeInt},
		info:           tfbridge.DefaultInfo{Value: 0x4eedface},
		expectedNode:   "1324219086",
		expectedGo:     "1324219086",
		expectedPython: "1324219086",
	},
	{
		schema:         schema.Schema{Type: schema.TypeInt},
		info:           tfbridge.DefaultInfo{Value: uint(0xfeedface)},
		expectedNode:   "4277009102",
		expectedGo:     "4277009102",
		expectedPython: "4277009102",
	},
	{
		schema:         schema.Schema{Type: schema.TypeFloat},
		info:           tfbridge.DefaultInfo{Value: math.Pi},
		expectedNode:   "3.141592653589793",
		expectedGo:     "3.141592653589793",
		expectedPython: "3.141592653589793",
	},
	{
		schema:         schema.Schema{Type: schema.TypeString},
		info:           tfbridge.DefaultInfo{Value: "foo"},
		expectedNode:   `"foo"`,
		expectedGo:     `"foo"`,
		expectedPython: `'foo'`,
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

func Test_GoDefaults(t *testing.T) {
	g := &goGenerator{}

	testVar := func(v *variable, dvt defaultValueTest) {
		v.info.Default.EnvVars = nil
		actual := g.goDefaultValue(v)
		assert.Equal(t, dvt.expectedGo, actual)

		parser, outDefault := "nil", "\"\""
		switch dvt.schema.Type {
		case schema.TypeBool:
			parser, outDefault = "parseEnvBool", "false"
		case schema.TypeInt:
			parser, outDefault = "parseEnvInt", "0"
		case schema.TypeFloat:
			parser, outDefault = "parseEnvFloat", "0.0"
		}

		defaultValue := dvt.expectedGo
		if defaultValue == "" {
			if v.out {
				defaultValue = outDefault
			} else {
				defaultValue = "nil"
			}
		}

		v.info.Default.EnvVars = []string{"FOO"}
		expected := fmt.Sprintf(`getEnvOrDefault(%s, %s, "FOO")`, defaultValue, parser)
		actual = g.goDefaultValue(v)
		assert.Equal(t, expected, actual)

		v.info.Default.EnvVars = []string{"FOO", "BAR"}
		expected = fmt.Sprintf(`getEnvOrDefault(%s, %s, "FOO", "BAR")`, defaultValue, parser)
		actual = g.goDefaultValue(v)
		assert.Equal(t, expected, actual)
	}

	for _, dvt := range defaultTests {
		v := &variable{
			name:   "v",
			schema: &dvt.schema,
			info:   &tfbridge.SchemaInfo{Default: &dvt.info},
		}

		testVar(v, dvt)
		v.out = true
		testVar(v, dvt)
	}
}

func Test_PythonDefaults(t *testing.T) {
	for _, dvt := range defaultTests {
		v := &variable{
			name:   "v",
			schema: &dvt.schema,
			info:   &tfbridge.SchemaInfo{Default: &dvt.info},
		}

		actual := pyDefaultValue(v)
		assert.Equal(t, dvt.expectedPython, actual)

		envFunc := "utilities.get_env"
		switch v.schema.Type {
		case schema.TypeBool:
			envFunc = "utilities.get_env_bool"
		case schema.TypeInt:
			envFunc = "utilities.get_env_int"
		case schema.TypeFloat:
			envFunc = "utilities.get_env_float"
		}

		singleEnv, multiEnv := fmt.Sprintf(`%s('FOO')`, envFunc), fmt.Sprintf(`%s('FOO', 'BAR')`, envFunc)
		if dvt.expectedPython != "" {
			singleEnv = fmt.Sprintf("(%s or %s)", singleEnv, dvt.expectedPython)
			multiEnv = fmt.Sprintf("(%s or %s)", multiEnv, dvt.expectedPython)
		}

		v.info.Default.EnvVars = []string{"FOO"}
		actual = pyDefaultValue(v)
		assert.Equal(t, singleEnv, actual)

		v.info.Default.EnvVars = []string{"FOO", "BAR"}
		actual = pyDefaultValue(v)
		assert.Equal(t, multiEnv, actual)
	}
}
