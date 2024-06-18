package tfbridgetests

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestNestedCustomTypeEncoding(t *testing.T) {

	testProvider := &providerbuilder.Provider{
		TypeName: "testprovider",
		Version:  "0.0.1",
		// This resource is modified from AWS Bedrockagent.
		AllResources: []providerbuilder.Resource{{
			Name: "bedrockagent",
			ResourceSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"prompt_override_configuration": schema.ListAttribute{ // proto5 Optional+Computed nested block.
						CustomType: NewListNestedObjectTypeOf[promptOverrideConfigurationModel](context.Background()),
						Optional:   true,
						Computed:   true,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
						Validators: []validator.List{
							listvalidator.SizeAtMost(1),
						},
						ElementType: types.ObjectType{
							AttrTypes: AttributeTypesMust[promptOverrideConfigurationModel](context.Background()),
						},
					},
				},
			},
		}},
	}
	res := tfbridge0.ResourceInfo{
		Tok: "testprovider:index/bedrockagent:Bedrockagent",
		Docs: &tfbridge0.DocInfo{
			Markdown: []byte("OK"),
		},
		Fields: map[string]*tfbridge0.SchemaInfo{},
	}

	info := tfbridge0.ProviderInfo{
		Name:         "testprovider",
		P:            tfbridge.ShimProvider(testProvider),
		Version:      "0.0.1",
		MetadataInfo: &tfbridge0.MetadataInfo{},
		Resources: map[string]*tfbridge0.ResourceInfo{
			"testprovider_bedrockagent": &res,
		},
	}

	encoding := convert.NewEncoding(info.P, &info)
	objType := convert.InferObjectType(info.P.ResourcesMap().Get("testprovider_bedrockagent").Schema(), nil)
	_, err := encoding.NewResourceEncoder("testprovider_bedrockagent", objType)
	assert.NoError(t, err)
}

type promptOverrideConfigurationModel struct {
	PromptConfigurations SetNestedObjectValueOf[promptConfigurationModel] `tfsdk:"prompt_configurations"`
}

type promptConfigurationModel struct {
	BasePromptTemplate types.String `tfsdk:"base_prompt_template"`
}

// Implementation for set, list, and object typables.

// Set

var (
	_ basetypes.SetTypable  = (*setNestedObjectTypeOf[struct{}])(nil)
	_ basetypes.SetValuable = (*SetNestedObjectValueOf[struct{}])(nil)
)

type setNestedObjectTypeOf[T any] struct {
	basetypes.SetType
}

type SetNestedObjectValueOf[T any] struct {
	basetypes.SetValue
}

func (setNested setNestedObjectTypeOf[T]) ValueFromSet(ctx context.Context, in basetypes.SetValue) (basetypes.SetValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return NewSetNestedObjectValueOfNull[T](ctx), diags
	}
	if in.IsUnknown() {
		return NewSetNestedObjectValueOfUnknown[T](ctx), diags
	}

	typ, d := newObjectTypeOf[T](ctx)
	diags.Append(d...)
	if diags.HasError() {
		return NewSetNestedObjectValueOfUnknown[T](ctx), diags
	}

	v, d := basetypes.NewSetValue(typ, in.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return NewSetNestedObjectValueOfUnknown[T](ctx), diags
	}

	return SetNestedObjectValueOf[T]{SetValue: v}, diags
}

func NewSetNestedObjectValueOfNull[T any](ctx context.Context) SetNestedObjectValueOf[T] {
	return SetNestedObjectValueOf[T]{SetValue: basetypes.NewSetNull(NewObjectTypeOf[T](ctx))}
}

func NewSetNestedObjectValueOfUnknown[T any](ctx context.Context) SetNestedObjectValueOf[T] {
	return SetNestedObjectValueOf[T]{SetValue: basetypes.NewSetUnknown(NewObjectTypeOf[T](ctx))}
}

func (t setNestedObjectTypeOf[T]) ValueType(ctx context.Context) attr.Value {
	return SetNestedObjectValueOf[T]{}
}

func (v SetNestedObjectValueOf[T]) Equal(o attr.Value) bool {
	other, ok := o.(SetNestedObjectValueOf[T])

	if !ok {
		return false
	}

	return v.SetValue.Equal(other.SetValue)
}

func (v SetNestedObjectValueOf[T]) Type(ctx context.Context) attr.Type {
	return NewSetNestedObjectTypeOf[T](ctx)
}
func NewSetNestedObjectTypeOf[T any](ctx context.Context) setNestedObjectTypeOf[T] {
	return setNestedObjectTypeOf[T]{basetypes.SetType{ElemType: NewObjectTypeOf[T](ctx)}}
}

/// List

var (
	_ basetypes.ListTypable  = (*listNestedObjectTypeOf[struct{}])(nil)
	_ basetypes.ListValuable = (*ListNestedObjectValueOf[struct{}])(nil)
)

// ListNestedObjectValueOf represents a Terraform Plugin Framework List value whose elements are of type `ObjectTypeOf[T]`.
type ListNestedObjectValueOf[T any] struct {
	basetypes.ListValue
}

// listNestedObjectTypeOf is the attribute type of a ListNestedObjectValueOf.
type listNestedObjectTypeOf[T any] struct {
	basetypes.ListType
}

func NewListNestedObjectTypeOf[T any](ctx context.Context) listNestedObjectTypeOf[T] {
	return listNestedObjectTypeOf[T]{basetypes.ListType{ElemType: NewObjectTypeOf[T](ctx)}}
}

func (listNested listNestedObjectTypeOf[T]) ValueFromList(ctx context.Context, listval basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if listval.IsNull() {
		return NewListNestedObjectValueOfNull[T](ctx), diags
	}
	if listval.IsUnknown() {
		return NewListNestedObjectValueOfUnknown[T](ctx), diags
	}

	typ, d := newObjectTypeOf[T](ctx)
	diags.Append(d...)
	if diags.HasError() {
		return NewListNestedObjectValueOfUnknown[T](ctx), diags
	}

	v, d := basetypes.NewListValue(typ, listval.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return NewListNestedObjectValueOfUnknown[T](ctx), diags
	}

	return ListNestedObjectValueOf[T]{ListValue: v}, diags
}

func NewListNestedObjectValueOfNull[T any](ctx context.Context) ListNestedObjectValueOf[T] {
	return ListNestedObjectValueOf[T]{ListValue: basetypes.NewListNull(NewObjectTypeOf[T](ctx))}
}

func NewListNestedObjectValueOfUnknown[T any](ctx context.Context) ListNestedObjectValueOf[T] {
	return ListNestedObjectValueOf[T]{ListValue: basetypes.NewListUnknown(NewObjectTypeOf[T](ctx))}
}

/// Object

var (
	_ basetypes.ObjectTypable  = (*objectTypeOf[struct{}])(nil)
	_ basetypes.ObjectValuable = (*ObjectValueOf[struct{}])(nil)
)

type objectTypeOf[T any] struct {
	basetypes.ObjectType
}

type ObjectValueOf[T any] struct {
	basetypes.ObjectValue
}

func newObjectTypeOf[T any](ctx context.Context) (objectTypeOf[T], diag.Diagnostics) {
	var diags diag.Diagnostics

	m, d := AttributeTypes[T](ctx)
	diags.Append(d...)
	if diags.HasError() {
		return objectTypeOf[T]{}, diags
	}

	return objectTypeOf[T]{basetypes.ObjectType{AttrTypes: m}}, diags
}

func NewObjectTypeOf[T any](ctx context.Context) basetypes.ObjectTypable {
	return objectTypeOf[T]{basetypes.ObjectType{AttrTypes: AttributeTypesMust[T](ctx)}}
}

func AttributeTypesMust[T any](ctx context.Context) map[string]attr.Type {
	return must(AttributeTypes[T](ctx))
}

func must[T any, E any](t T, err E) T {
	if v := reflect.ValueOf(err); v.IsValid() && !v.IsZero() {
		panic(err)
	}
	return t
}

// AttributeTypes returns a map of attribute types for the specified type T.
// T must be a struct and reflection is used to find exported fields of T with the `tfsdk` tag.
func AttributeTypes[T any](ctx context.Context) (map[string]attr.Type, diag.Diagnostics) {
	var diags diag.Diagnostics
	var t T
	val := reflect.ValueOf(t)
	typ := val.Type()

	if typ.Kind() == reflect.Ptr && typ.Elem().Kind() == reflect.Struct {
		val = reflect.New(typ.Elem()).Elem()
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		diags.Append(diag.NewErrorDiagnostic("Invalid type", fmt.Sprintf("%T has unsupported type: %s", t, typ)))
		return nil, diags
	}

	attributeTypes := make(map[string]attr.Type)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue // Skip unexported fields.
		}
		tag := field.Tag.Get(`tfsdk`)
		if tag == "-" {
			continue // Skip explicitly excluded fields.
		}
		if tag == "" {
			diags.Append(diag.NewErrorDiagnostic("Invalid type", fmt.Sprintf(`%T needs a struct tag for "tfsdk" on %s`, t, field.Name)))
			return nil, diags
		}

		if v, ok := val.Field(i).Interface().(attr.Value); ok {
			attributeTypes[tag] = v.Type(ctx)
		}
	}

	return attributeTypes, nil
}
