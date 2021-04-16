package tfplugin5

import (
	"bytes"
	goerrors "errors"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type testLogger struct {
	t     *testing.T
	level hclog.Level
	name  string
	args  []interface{}
}

func (l *testLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	var buf bytes.Buffer
	hcl := hclog.New(&hclog.LoggerOptions{
		Name:   l.name,
		Level:  l.level,
		Output: &buf,
	})
	if len(l.args) != 0 {
		hcl = hcl.With(l.args...)
	}
	switch level {
	case hclog.Trace:
		hcl.Trace(msg, args...)
	case hclog.Debug:
		hcl.Debug(msg, args...)
	case hclog.Info:
		hcl.Info(msg, args...)
	case hclog.Warn:
		hcl.Warn(msg, args...)
	case hclog.Error:
		hcl.Error(msg, args...)
	}
	l.t.Log(buf.String())
}

func (l *testLogger) Trace(msg string, args ...interface{}) {
	l.Log(hclog.Trace, msg, args...)
}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	l.Log(hclog.Debug, msg, args...)
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	l.Log(hclog.Info, msg, args...)
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
	l.Log(hclog.Warn, msg, args...)
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	l.Log(hclog.Error, msg, args...)
}

func (l *testLogger) IsTrace() bool {
	return l.level <= hclog.Trace
}

func (l *testLogger) IsDebug() bool {
	return l.level <= hclog.Debug
}

func (l *testLogger) IsInfo() bool {
	return l.level <= hclog.Info
}

func (l *testLogger) IsWarn() bool {
	return l.level <= hclog.Warn
}

func (l *testLogger) IsError() bool {
	return l.level <= hclog.Error
}

func (l *testLogger) ImpliedArgs() []interface{} {
	return l.args
}

func (l *testLogger) With(args ...interface{}) hclog.Logger {
	return &testLogger{
		t:    l.t,
		name: l.name,
		args: append(l.args, args...),
	}
}

func (l *testLogger) Name() string {
	return l.name
}

func (l *testLogger) Named(name string) hclog.Logger {
	return &testLogger{
		t:    l.t,
		name: l.name + " " + name,
		args: l.args,
	}
}

func (l *testLogger) ResetNamed(name string) hclog.Logger {
	return &testLogger{
		t:    l.t,
		name: name,
		args: l.args,
	}
}

func (l *testLogger) SetLevel(level hclog.Level) {
	// Do nothing.
}

func (l *testLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	panic("unsupported")
}

func (l *testLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	panic("unsupported")
}

func startTestProvider(t *testing.T) (*provider, bool) {
	testProviderPath, err := exec.LookPath("pulumi-terraform-bridge-test-provider")
	if !assert.NoError(t, err) {
		return nil, false
	}

	var logger hclog.Logger
	switch os.Getenv("TF_LOG") {
	case "TRACE":
		logger = &testLogger{t: t, level: hclog.Trace}
	case "DEBUG":
		logger = &testLogger{t: t, level: hclog.Debug}
	case "INFO":
		logger = &testLogger{t: t, level: hclog.Info}
	case "WARN":
		logger = &testLogger{t: t, level: hclog.Warn}
	case "ERROR":
		logger = &testLogger{t: t, level: hclog.Error}
	default:
		logger = hclog.NewNullLogger()
	}

	pluginClient := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          goplugin.PluginSet{"provider": &providerPlugin{}},
		Cmd:              exec.Command(testProviderPath),
		Managed:          true,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		AutoMTLS:         true,
		Logger:           logger,
	})
	client, err := pluginClient.Client()
	if !assert.NoError(t, err) {
		return nil, false
	}
	p, err := client.Dispense("provider")
	if !assert.NoError(t, err) {
		return nil, false
	}

	provider := p.(*provider)
	t.Cleanup(func() {
		err := provider.Stop()
		contract.IgnoreError(err)

		pluginClient.Kill()
	})

	return provider, true
}

func TestProviderSchema(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	properties := map[string]*attributeSchema{}
	p.Schema().Range(func(k string, v shim.Schema) bool {
		properties[k] = v.(*attributeSchema)
		return true
	})
	assert.Equal(t, map[string]*attributeSchema{
		"config_value": {
			ctyType:   cty.String,
			valueType: shim.TypeString,
			optional:  true,
		},
	}, properties)
}

func TestProviderResourcesMap(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	expected := map[string]*resource{
		"example_resource": {
			resourceType:  "example_resource",
			schemaVersion: 1,
			ctyType: cty.Object(map[string]cty.Type{
				"id": cty.String,
				"timeouts": cty.Object(map[string]cty.Type{
					"create": cty.String,
				}),
				"nil_property_value":    cty.Map(cty.String),
				"bool_property_value":   cty.Bool,
				"number_property_value": cty.Number,
				"float_property_value":  cty.Number,
				"string_property_value": cty.String,
				"array_property_value":  cty.List(cty.String),
				"object_property_value": cty.Map(cty.String),
				"nested_resources": cty.List(cty.Object(map[string]cty.Type{
					"kind":          cty.String,
					"configuration": cty.Map(cty.String),
				})),
				"set_property_value":            cty.Set(cty.String),
				"string_with_bad_interpolation": cty.String,
			}),
			schema: schema.SchemaMap{
				"id": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					computed:  true,
				},
				"timeouts": &attributeSchema{
					ctyType: cty.Object(map[string]cty.Type{
						"create": cty.String,
					}),
					valueType: shim.TypeMap,
					elem: &resource{
						ctyType: cty.Object(map[string]cty.Type{
							"create": cty.String,
						}),
						schema: schema.SchemaMap{
							"create": &attributeSchema{
								ctyType:   cty.String,
								valueType: shim.TypeString,
								optional:  true,
							},
						},
					},
					required: true,
				},
				"nil_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"bool_property_value": &attributeSchema{
					ctyType:   cty.Bool,
					valueType: shim.TypeBool,
					optional:  true,
				},
				"number_property_value": &attributeSchema{
					ctyType:   cty.Number,
					valueType: shim.TypeFloat,
					optional:  true,
				},
				"float_property_value": &attributeSchema{
					ctyType:   cty.Number,
					valueType: shim.TypeFloat,
					optional:  true,
				},
				"string_property_value": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
				"array_property_value": &attributeSchema{
					ctyType:   cty.List(cty.String),
					valueType: shim.TypeList,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					required: true,
				},
				"object_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"nested_resources": &attributeSchema{
					ctyType: cty.List(cty.Object(map[string]cty.Type{
						"kind":          cty.String,
						"configuration": cty.Map(cty.String),
					})),
					valueType: shim.TypeList,
					elem: &resource{
						ctyType: cty.Object(map[string]cty.Type{
							"kind":          cty.String,
							"configuration": cty.Map(cty.String),
						}),
						schema: schema.SchemaMap{
							"kind": &attributeSchema{
								ctyType:   cty.String,
								valueType: shim.TypeString,
								optional:  true,
							},
							"configuration": &attributeSchema{
								ctyType:   cty.Map(cty.String),
								valueType: shim.TypeMap,
								elem: &attributeSchema{
									ctyType:   cty.String,
									valueType: shim.TypeString,
								},
								required: true,
							},
						},
					},
					maxItems: 1,
					optional: true,
				},
				"set_property_value": &attributeSchema{
					ctyType:   cty.Set(cty.String),
					valueType: shim.TypeSet,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"string_with_bad_interpolation": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
			},
		},
		"second_resource": {
			resourceType:  "second_resource",
			schemaVersion: 1,
			ctyType: cty.Object(map[string]cty.Type{
				"id": cty.String,
				"timeouts": cty.Object(map[string]cty.Type{
					"create": cty.String,
					"update": cty.String,
				}),
				"nil_property_value":    cty.Map(cty.String),
				"bool_property_value":   cty.Bool,
				"number_property_value": cty.Number,
				"float_property_value":  cty.Number,
				"string_property_value": cty.String,
				"array_property_value":  cty.List(cty.String),
				"object_property_value": cty.Map(cty.String),
				"nested_resources": cty.List(cty.Object(map[string]cty.Type{
					"configuration": cty.Map(cty.String),
				})),
				"set_property_value":            cty.Set(cty.String),
				"string_with_bad_interpolation": cty.String,
				"conflicting_property":          cty.String,
				"conflicting_property2":         cty.String,
			}),
			schema: schema.SchemaMap{
				"id": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					computed:  true,
				},
				"timeouts": &attributeSchema{
					ctyType: cty.Object(map[string]cty.Type{
						"create": cty.String,
						"update": cty.String,
					}),
					valueType: shim.TypeMap,
					elem: &resource{
						ctyType: cty.Object(map[string]cty.Type{
							"create": cty.String,
							"update": cty.String,
						}),
						schema: schema.SchemaMap{
							"create": &attributeSchema{
								ctyType:   cty.String,
								valueType: shim.TypeString,
								optional:  true,
							},
							"update": &attributeSchema{
								ctyType:   cty.String,
								valueType: shim.TypeString,
								optional:  true,
							},
						},
					},
					required: true,
				},
				"nil_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"bool_property_value": &attributeSchema{
					ctyType:   cty.Bool,
					valueType: shim.TypeBool,
					optional:  true,
				},
				"number_property_value": &attributeSchema{
					ctyType:   cty.Number,
					valueType: shim.TypeFloat,
					optional:  true,
				},
				"float_property_value": &attributeSchema{
					ctyType:   cty.Number,
					valueType: shim.TypeFloat,
					optional:  true,
				},
				"string_property_value": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
				"array_property_value": &attributeSchema{
					ctyType:   cty.List(cty.String),
					valueType: shim.TypeList,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					required: true,
				},
				"object_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"nested_resources": &attributeSchema{
					ctyType: cty.List(cty.Object(map[string]cty.Type{
						"configuration": cty.Map(cty.String),
					})),
					valueType: shim.TypeList,
					elem: &resource{
						ctyType: cty.Object(map[string]cty.Type{
							"configuration": cty.Map(cty.String),
						}),
						schema: schema.SchemaMap{
							"configuration": &attributeSchema{
								ctyType:   cty.Map(cty.String),
								valueType: shim.TypeMap,
								elem: &attributeSchema{
									ctyType:   cty.String,
									valueType: shim.TypeString,
								},
								required: true,
							},
						},
					},
					maxItems: 1,
					optional: true,
				},
				"set_property_value": &attributeSchema{
					ctyType:   cty.Set(cty.String),
					valueType: shim.TypeSet,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"string_with_bad_interpolation": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
				"conflicting_property": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
				"conflicting_property2": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
			},
		},
	}

	names := map[string]bool{}
	p.ResourcesMap().Range(func(name string, v shim.Resource) bool {
		expected, ok := expected[name]
		if !assert.Truef(t, ok, "extra resource %v", name) {
			return true
		}
		names[name] = true

		// Ignore the provider field of both resources.
		actual := v.(*resource)
		assert.Equal(t, expected.resourceType, actual.resourceType)
		assert.Equal(t, expected.ctyType, actual.ctyType)
		assert.Equal(t, expected.schema, actual.schema)
		assert.Equal(t, expected.schemaVersion, actual.schemaVersion)
		return true
	})

	for name := range expected {
		_, ok := names[name]
		assert.Truef(t, ok, "missing resource %v", name)
	}
}

func TestProviderDataSourcesMap(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	expected := map[string]*resource{
		"example_resource": {
			resourceType:  "example_resource",
			schemaVersion: 1,
			ctyType: cty.Object(map[string]cty.Type{
				"id":                    cty.String,
				"nil_property_value":    cty.Map(cty.String),
				"bool_property_value":   cty.Bool,
				"number_property_value": cty.Number,
				"float_property_value":  cty.Number,
				"string_property_value": cty.String,
				"array_property_value":  cty.List(cty.String),
				"object_property_value": cty.Map(cty.String),
				"map_property_value":    cty.Map(cty.String),
				"nested_resources": cty.List(cty.Object(map[string]cty.Type{
					"configuration": cty.Map(cty.String),
				})),
				"set_property_value":            cty.Set(cty.String),
				"string_with_bad_interpolation": cty.String,
			}),
			schema: schema.SchemaMap{
				"id": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					computed:  true,
				},
				"nil_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"bool_property_value": &attributeSchema{
					ctyType:   cty.Bool,
					valueType: shim.TypeBool,
					optional:  true,
				},
				"number_property_value": &attributeSchema{
					ctyType:   cty.Number,
					valueType: shim.TypeFloat,
					optional:  true,
				},
				"float_property_value": &attributeSchema{
					ctyType:   cty.Number,
					valueType: shim.TypeFloat,
					optional:  true,
				},
				"string_property_value": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
				"array_property_value": &attributeSchema{
					ctyType:   cty.List(cty.String),
					valueType: shim.TypeList,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					required: true,
				},
				"object_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"map_property_value": &attributeSchema{
					ctyType:   cty.Map(cty.String),
					valueType: shim.TypeMap,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"nested_resources": &attributeSchema{
					ctyType: cty.List(cty.Object(map[string]cty.Type{
						"configuration": cty.Map(cty.String),
					})),
					valueType: shim.TypeList,
					elem: &resource{
						ctyType: cty.Object(map[string]cty.Type{
							"configuration": cty.Map(cty.String),
						}),
						schema: schema.SchemaMap{
							"configuration": &attributeSchema{
								ctyType:   cty.Map(cty.String),
								valueType: shim.TypeMap,
								elem: &attributeSchema{
									ctyType:   cty.String,
									valueType: shim.TypeString,
								},
								required: true,
							},
						},
					},
					maxItems: 1,
					optional: true,
				},
				"set_property_value": &attributeSchema{
					ctyType:   cty.Set(cty.String),
					valueType: shim.TypeSet,
					elem: &attributeSchema{
						ctyType:   cty.String,
						valueType: shim.TypeString,
					},
					optional: true,
				},
				"string_with_bad_interpolation": &attributeSchema{
					ctyType:   cty.String,
					valueType: shim.TypeString,
					optional:  true,
				},
			},
		},
	}

	names := map[string]bool{}
	p.DataSourcesMap().Range(func(name string, v shim.Resource) bool {
		expected, ok := expected[name]
		if !assert.Truef(t, ok, "extra data source %v", name) {
			return true
		}
		names[name] = true

		// Ignore the provider field of both resources.
		actual := v.(*resource)
		assert.Equal(t, expected.resourceType, actual.resourceType)
		assert.Equal(t, expected.ctyType, actual.ctyType)
		assert.Equal(t, expected.schema, actual.schema)
		assert.Equal(t, expected.schemaVersion, actual.schemaVersion)
		return true
	})

	for name := range expected {
		_, ok := names[name]
		assert.Truef(t, ok, "missing data source %v", name)
	}
}

func TestValidate(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	warnings, errors := p.Validate(p.NewResourceConfig(map[string]interface{}{
		"config_value": "foo",
	}))
	assert.Empty(t, warnings)
	assert.Empty(t, errors)
}

func TestValidateResource(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	warnings, errors := p.ValidateResource("example_resource", p.NewResourceConfig(map[string]interface{}{}))
	assert.Empty(t, warnings)
	assert.NotEmpty(t, errors)

	warnings, errors = p.ValidateResource("example_resource", p.NewResourceConfig(map[string]interface{}{
		"array_property_value": []interface{}{},
	}))
	assert.Empty(t, warnings)
	assert.Empty(t, errors)

	warnings, errors = p.ValidateResource("example_resource", p.NewResourceConfig(map[string]interface{}{
		"nil_property_value":    map[string]interface{}{"foo": "bar"},
		"bool_property_value":   true,
		"number_property_value": 42,
		"float_property_value":  float64(3.14),
		"string_property_value": "foo",
		"array_property_value":  []interface{}{"baz"},
		"object_property_value": map[string]interface{}{"qux": "zed"},
		"nested_resources": []interface{}{
			map[string]interface{}{
				"configuration": map[string]interface{}{"alpha": "beta"},
			},
		},
		"set_property_value":            []interface{}{"gamma"},
		"string_with_bad_interpolation": "delta",
	}))
	assert.Empty(t, warnings)
	assert.Empty(t, errors)

	var err *diagnostics.ValidationError
	warnings, errors = p.ValidateResource("example_resource", p.NewResourceConfig(map[string]interface{}{
		// missing required array_property_value
	}))
	assert.Empty(t, warnings)
	assert.Len(t, errors, 1)
	if goerrors.As(errors[0], &err) {
		assert.Equal(t, &diagnostics.ValidationError{
			Summary:       "Required attribute is not set",
			AttributePath: cty.GetAttrPath("array_property_value"),
		}, err)
	} else {
		t.Error("Validate missing required property")
	}
}

func TestValidateDataSource(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	warnings, errors := p.ValidateDataSource("example_resource", p.NewResourceConfig(map[string]interface{}{}))
	assert.Empty(t, warnings)
	assert.NotEmpty(t, errors)

	warnings, errors = p.ValidateDataSource("example_resource", p.NewResourceConfig(map[string]interface{}{
		"array_property_value": []interface{}{},
	}))
	assert.Empty(t, warnings)
	assert.Empty(t, errors)

	warnings, errors = p.ValidateDataSource("example_resource", p.NewResourceConfig(map[string]interface{}{
		"nil_property_value":    map[string]interface{}{"foo": "bar"},
		"bool_property_value":   true,
		"number_property_value": 42,
		"float_property_value":  float64(3.14),
		"string_property_value": "foo",
		"array_property_value":  []interface{}{"baz"},
		"object_property_value": map[string]interface{}{"qux": "zed"},
		"nested_resources": []interface{}{
			map[string]interface{}{
				"configuration": map[string]interface{}{"alpha": "beta"},
			},
		},
		"set_property_value":            []interface{}{"gamma"},
		"string_with_bad_interpolation": "delta",
	}))
	assert.Empty(t, warnings)
	assert.Empty(t, errors)
}

func TestConfigure(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	err := p.Configure(p.NewResourceConfig(map[string]interface{}{
		"config_value": "foo",
	}))
	assert.NoError(t, err)
}

func TestDiff(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	resource, ok := p.ResourcesMap().GetOk("example_resource")
	if !assert.True(t, ok) {
		return
	}

	err := p.Configure(p.NewResourceConfig(map[string]interface{}{
		"config_value": "foo",
	}))
	if !assert.NoError(t, err) {
		return
	}

	cases := []struct {
		state      map[string]interface{}
		config     map[string]interface{}
		attributes map[string]shim.ResourceAttrDiff
	}{
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			attributes: map[string]shim.ResourceAttrDiff{},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value":  []interface{}{"bar"},
				"number_property_value": 42,
			},
			attributes: map[string]shim.ResourceAttrDiff{
				"array_property_value.0": update("foo", "bar", false),
				"number_property_value":  add("42", false),
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value":  []interface{}{"bar"},
				"number_property_value": UnknownVariableValue,
			},
			attributes: map[string]shim.ResourceAttrDiff{
				"array_property_value.0": update("foo", "bar", false),
				"number_property_value":  add(UnknownVariableValue, false),
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"set_property_value":   []interface{}{"bar"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"set_property_value":   []interface{}{"baz"},
			},
			attributes: map[string]shim.ResourceAttrDiff{
				"set_property_value.1836076918": remove("bar", true),
				"set_property_value.2779366782": add("baz", true),
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"set_property_value":   []interface{}{"bar"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"set_property_value":   []interface{}{UnknownVariableValue},
			},
			attributes: map[string]shim.ResourceAttrDiff{
				"set_property_value.#": update("1", UnknownVariableValue, true),
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"nested_resources": []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"baz": "qux",
						},
					},
				},
			},
			attributes: map[string]shim.ResourceAttrDiff{
				"nested_resources.#":                   update("0", "1", false),
				"nested_resources.0.configuration.%":   add("1", false),
				"nested_resources.0.configuration.baz": add("qux", false),
			},
		},
	}
	for i, c := range cases {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			expected := map[string]cty.Value{
				"id": cty.StringVal("0"),
				"timeouts": cty.NullVal(cty.Object(map[string]cty.Type{
					"create": cty.String,
				})),
				"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
				"bool_property_value":   cty.NullVal(cty.Bool),
				"number_property_value": cty.NullVal(cty.Number),
				"float_property_value":  cty.NullVal(cty.Number),
				"string_property_value": cty.NullVal(cty.String),
				"array_property_value":  cty.ListValEmpty(cty.String),
				"object_property_value": cty.NullVal(cty.Map(cty.String)),
				"nested_resources": cty.ListValEmpty(cty.Object(map[string]cty.Type{
					"kind":          cty.String,
					"configuration": cty.Map(cty.String),
				})),
				"set_property_value":            cty.NullVal(cty.Set(cty.String)),
				"string_with_bad_interpolation": cty.NullVal(cty.String),
			}
			for k, v := range c.state {
				val, err := goToCty(v, expected[k].Type())
				if !assert.NoError(t, err) {
					return
				}
				expected[k] = val
			}
			for k, v := range c.config {
				val, err := goToCty(v, expected[k].Type())
				if !assert.NoError(t, err) {
					return
				}
				expected[k] = val
			}

			requiresNew := false
			for _, d := range c.attributes {
				if d.RequiresNew {
					requiresNew = true
					break
				}
			}

			state, err := resource.InstanceState("0", c.state, nil)
			if !assert.NoError(t, err) {
				return
			}

			config := p.NewResourceConfig(c.config)

			diff, err := p.Diff("example_resource", state, config)
			if !assert.NoError(t, err) {
				return
			}

			var meta map[string]interface{}
			if len(c.attributes) != 0 {
				meta = map[string]interface{}{
					"_new_extra_shim": map[string]interface{}{},
					timeoutsKey: map[string]interface{}{
						"create": float64(1.2e11),
					},
				}
			}

			assert.Equal(t, &instanceDiff{
				planned:     cty.ObjectVal(expected),
				attributes:  c.attributes,
				requiresNew: requiresNew,
				meta:        meta,
			}, diff)
		})
	}
}

func TestApply(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	resource, ok := p.ResourcesMap().GetOk("example_resource")
	if !assert.True(t, ok) {
		return
	}

	err := p.Configure(p.NewResourceConfig(map[string]interface{}{
		"config_value": "foo",
	}))
	if !assert.NoError(t, err) {
		return
	}

	cases := []struct {
		state  map[string]interface{}
		config map[string]interface{}
	}{
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value":  []interface{}{"bar"},
				"number_property_value": 42,
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"set_property_value":   []interface{}{"bar"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"set_property_value":   []interface{}{"baz"},
			},
		},
		{
			state: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
			},
			config: map[string]interface{}{
				"array_property_value": []interface{}{"foo"},
				"nested_resources": []interface{}{
					map[string]interface{}{
						"kind": cty.StringVal(""),
						"configuration": map[string]interface{}{
							"baz": "qux",
						},
					},
				},
			},
		},
	}
	for i, c := range cases {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			expected := map[string]cty.Value{
				"id": cty.StringVal("0"),
				"timeouts": cty.NullVal(cty.Object(map[string]cty.Type{
					"create": cty.String,
				})),
				"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
				"bool_property_value":   cty.False,
				"number_property_value": cty.NumberIntVal(42),
				"float_property_value":  cty.NumberFloatVal(99.6767932),
				"string_property_value": cty.StringVal("ognirts"),
				"array_property_value":  cty.ListVal([]cty.Value{cty.StringVal("an array")}),
				"object_property_value": cty.MapVal(map[string]cty.Value{
					"property_a": cty.StringVal("a"),
					"property_b": cty.StringVal("true"),
					"property.c": cty.StringVal("some.value"),
				}),
				"nested_resources": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"kind": cty.StringVal(""),
					"configuration": cty.MapVal(map[string]cty.Value{
						"configurationValue": cty.StringVal("true"),
					}),
				})}),
				"set_property_value": cty.SetVal([]cty.Value{
					cty.StringVal("set member 1"),
					cty.StringVal("set member 2"),
				}),
				"string_with_bad_interpolation": cty.StringVal("some ${interpolated:value} with syntax errors"),
			}
			for k, v := range c.state {
				val, err := goToCty(v, expected[k].Type())
				if !assert.NoError(t, err) {
					return
				}
				expected[k] = val
			}
			for k, v := range c.config {
				val, err := goToCty(v, expected[k].Type())
				if !assert.NoError(t, err) {
					return
				}
				expected[k] = val
			}

			state, err := resource.InstanceState("0", c.state, nil)
			if !assert.NoError(t, err) {
				return
			}

			config := p.NewResourceConfig(c.config)

			diff, err := p.Diff("example_resource", state, config)
			if !assert.NoError(t, err) {
				return
			}

			if len(diff.Attributes()) == 0 {
				return
			}

			if diff.RequiresNew() {
				expected = map[string]cty.Value{
					"id": cty.StringVal("0"),
					"timeouts": cty.NullVal(cty.Object(map[string]cty.Type{
						"create": cty.String,
					})),
					"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
					"bool_property_value":   cty.False,
					"number_property_value": cty.NumberIntVal(42),
					"float_property_value":  cty.NumberFloatVal(99.6767932),
					"string_property_value": cty.StringVal("ognirts"),
					"array_property_value":  cty.ListVal([]cty.Value{cty.StringVal("an array")}),
					"object_property_value": cty.MapVal(map[string]cty.Value{
						"property_a": cty.StringVal("a"),
						"property_b": cty.StringVal("true"),
						"property.c": cty.StringVal("some.value"),
					}),
					"nested_resources": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
						"kind": cty.StringVal(""),
						"configuration": cty.MapVal(map[string]cty.Value{
							"configurationValue": cty.StringVal("true"),
						}),
					})}),
					"set_property_value": cty.SetVal([]cty.Value{
						cty.StringVal("set member 1"),
						cty.StringVal("set member 2"),
					}),
					"string_with_bad_interpolation": cty.StringVal("some ${interpolated:value} with syntax errors"),
				}
				for k, v := range c.config {
					val, err := goToCty(v, expected[k].Type())
					if !assert.NoError(t, err) {
						return
					}
					expected[k] = val
				}

				state, err = resource.InstanceState("", map[string]interface{}{}, nil)
				if !assert.NoError(t, err) {
					return
				}

				diff, err = p.Diff("example_resource", state, config)
				if !assert.NoError(t, err) {
					return
				}
			}

			state, err = p.Apply("example_resource", state, diff)
			if !assert.NoError(t, err) {
				return
			}

			expectedObject, err := ctyToGo(cty.ObjectVal(expected))
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, &instanceState{
				resourceType: "example_resource",
				id:           "0",
				object:       expectedObject.(map[string]interface{}),
				meta: map[string]interface{}{
					timeoutsKey: map[string]interface{}{
						"create": float64(1.2e11),
					},
					"schema_version": "1",
				},
			}, state)
		})
	}
}

func TestRefresh(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	resource, ok := p.ResourcesMap().GetOk("example_resource")
	if !assert.True(t, ok) {
		return
	}

	err := p.Configure(p.NewResourceConfig(map[string]interface{}{
		"config_value": "foo",
	}))
	if !assert.NoError(t, err) {
		return
	}

	meta := map[string]interface{}{
		timeoutsKey: map[string]interface{}{
			"create": float64(1.2e11),
		},
		"schema_version": "1",
	}

	state, err := resource.InstanceState("0", map[string]interface{}{}, meta)
	if !assert.NoError(t, err) {
		return
	}

	expected := map[string]cty.Value{
		"id": cty.StringVal("0"),
		"timeouts": cty.NullVal(cty.Object(map[string]cty.Type{
			"create": cty.String,
		})),
		"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
		"bool_property_value":   cty.False,
		"number_property_value": cty.NumberIntVal(42),
		"float_property_value":  cty.NumberFloatVal(99.6767932),
		"string_property_value": cty.StringVal("ognirts"),
		"array_property_value":  cty.ListVal([]cty.Value{cty.StringVal("an array")}),
		"object_property_value": cty.MapVal(map[string]cty.Value{
			"property_a": cty.StringVal("a"),
			"property_b": cty.StringVal("true"),
			"property.c": cty.StringVal("some.value"),
		}),
		"nested_resources": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"kind": cty.StringVal(""),
			"configuration": cty.MapVal(map[string]cty.Value{
				"configurationValue": cty.StringVal("true"),
			}),
		})}),
		"set_property_value": cty.SetVal([]cty.Value{
			cty.StringVal("set member 1"),
			cty.StringVal("set member 2"),
		}),
		"string_with_bad_interpolation": cty.StringVal("some ${interpolated:value} with syntax errors"),
	}

	state, err = p.Refresh("example_resource", state)
	if !assert.NoError(t, err) {
		return
	}

	expectedObject, err := ctyToGo(cty.ObjectVal(expected))
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, &instanceState{
		resourceType: "example_resource",
		id:           "0",
		object:       expectedObject.(map[string]interface{}),
		meta:         meta,
	}, state)
}

func TestReadDataDiff(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	config := p.NewResourceConfig(map[string]interface{}{
		"array_property_value": []interface{}{"foo"},
	})

	diff, err := p.ReadDataDiff("example_resource", config)
	if !assert.NoError(t, err) {
		return
	}

	expected := cty.ObjectVal(map[string]cty.Value{
		"id":                    cty.NullVal(cty.String),
		"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
		"bool_property_value":   cty.NullVal(cty.Bool),
		"number_property_value": cty.NullVal(cty.Number),
		"float_property_value":  cty.NullVal(cty.Number),
		"string_property_value": cty.NullVal(cty.String),
		"array_property_value":  cty.ListVal([]cty.Value{cty.StringVal("foo")}),
		"object_property_value": cty.NullVal(cty.Map(cty.String)),
		"map_property_value":    cty.NullVal(cty.Map(cty.String)),
		"nested_resources": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
			"configuration": cty.Map(cty.String),
		}))),
		"set_property_value":            cty.NullVal(cty.Set(cty.String)),
		"string_with_bad_interpolation": cty.NullVal(cty.String),
	})

	assert.Equal(t, &instanceDiff{planned: expected}, diff)
}

func TestReadDataApply(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	config := p.NewResourceConfig(map[string]interface{}{
		"array_property_value": []interface{}{"foo"},
	})

	diff, err := p.ReadDataDiff("example_resource", config)
	if !assert.NoError(t, err) {
		return
	}

	state, err := p.ReadDataApply("example_resource", diff)
	if !assert.NoError(t, err) {
		return
	}

	expected := cty.ObjectVal(map[string]cty.Value{
		"id":                    cty.StringVal("0"),
		"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
		"bool_property_value":   cty.False,
		"number_property_value": cty.NumberIntVal(42),
		"float_property_value":  cty.NumberFloatVal(99.6767932),
		"string_property_value": cty.StringVal("ognirts"),
		"array_property_value":  cty.ListVal([]cty.Value{cty.StringVal("foo")}),
		"object_property_value": cty.MapVal(map[string]cty.Value{
			"property_a": cty.StringVal("a"),
			"property_b": cty.StringVal("true"),
			"property.c": cty.StringVal("some.value"),
		}),
		"map_property_value": cty.NullVal(cty.Map(cty.String)),
		"nested_resources": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"configuration": cty.MapVal(map[string]cty.Value{
				"configurationValue": cty.StringVal("true"),
			}),
		})}),
		"set_property_value": cty.SetVal([]cty.Value{
			cty.StringVal("set member 1"),
			cty.StringVal("set member 2"),
		}),
		"string_with_bad_interpolation": cty.StringVal("some ${interpolated:value} with syntax errors"),
	})
	expectedObject, err := ctyToGo(expected)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, &instanceState{
		resourceType: "example_resource",
		id:           "0",
		object:       expectedObject.(map[string]interface{}),
	}, state)
}

func TestImportResourceState(t *testing.T) {
	p, ok := startTestProvider(t)
	if !ok {
		return
	}

	resource, ok := p.ResourcesMap().GetOk("example_resource")
	if !assert.True(t, ok) {
		return
	}

	importer := resource.Importer()
	if !assert.NotNil(t, importer) {
		return
	}

	states, err := importer("example_resource", "0", nil)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, states, 1) {
		return
	}
	state := states[0]

	expected := cty.ObjectVal(map[string]cty.Value{
		"id": cty.StringVal("0"),
		"timeouts": cty.ObjectVal(map[string]cty.Value{
			"create": cty.NullVal(cty.String),
		}),
		"nil_property_value":    cty.NullVal(cty.Map(cty.String)),
		"bool_property_value":   cty.NullVal(cty.Bool),
		"number_property_value": cty.NullVal(cty.Number),
		"float_property_value":  cty.NullVal(cty.Number),
		"string_property_value": cty.NullVal(cty.String),
		"array_property_value":  cty.NullVal(cty.List(cty.String)),
		"object_property_value": cty.NullVal(cty.Map(cty.String)),
		"nested_resources": cty.ListValEmpty(cty.Object(map[string]cty.Type{
			"configuration": cty.Map(cty.String),
		})),
		"set_property_value":            cty.NullVal(cty.Set(cty.String)),
		"string_with_bad_interpolation": cty.NullVal(cty.String),
	})
	expectedObject, err := ctyToGo(expected)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, &instanceState{
		resourceType: "example_resource",
		id:           "0",
		object:       expectedObject.(map[string]interface{}),
		meta: map[string]interface{}{
			timeoutsKey: map[string]interface{}{
				"create": float64(1.2e11),
			},
			"schema_version": "1",
		},
	}, state)
}
