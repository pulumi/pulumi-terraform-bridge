package providerbuilder

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"reflect"
	"testing"
)

func TestConversion(t *testing.T) {

	testProvider := &Provider{
		TypeName: "testprovider",
		Version:  "0.0.1",
		//ProviderSchema: pschema.Schema{
		//	Attributes: map[string]pschema.Attribute{
		//		"prop": pschema.StringAttribute{
		//			Optional: true,
		//		},
		//	},
		//},
		AllResources: []Resource{{
			Name: "res",
			ResourceSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"agent_version": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"prompt_override_configuration": schema.ListAttribute{ // proto5 Optional+Computed nested block.
						CustomType: newListNestedObjectTypeOf[promptOverrideConfigurationModel](context.Background()),
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
		Tok: "testprovider:index/res:Res",
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
			"prompt_override_configuration": &res,
		},
	}

	encoding := convert.NewEncoding(info.P, &info)
	objType := convert.InferObjectType(info.P.ResourcesMap().Get("prompt_override_configuration").Schema(), nil)
	encoding.NewResourceEncoder("prompt_override_configuration", objType)

}

type promptOverrideConfigurationModel struct {
	PromptConfigurations setNestedObjectValueOf[promptConfigurationModel] `tfsdk:"prompt_configurations"`
}

type promptConfigurationModel struct {
	BasePromptTemplate     types.String                                         `tfsdk:"base_prompt_template"`
	InferenceConfiguration listNestedObjectValueOf[inferenceConfigurationModel] `tfsdk:"inference_configuration"`
}

type inferenceConfigurationModel struct {
	MaximumLength types.Int64 `tfsdk:"max_length"`
}

type setNestedObjectValueOf[T any] struct {
	basetypes.SetValue
}

type listNestedObjectValueOf[T any] struct {
	basetypes.ListValue
}

// listNestedObjectTypeOf is the attribute type of a ListNestedObjectValueOf.
type listNestedObjectTypeOf[T any] struct {
	basetypes.ListType
}

type setNestedObjectTypeOf[T any] struct {
	basetypes.SetType
}

type objectTypeOf[T any] struct {
	basetypes.ObjectType
}

//func (s setNestedObjectTypeOf[T]) ValueFromList(ctx context.Context, value basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
//	//TODO implement me
//	panic("implement me")
//}

func (t listNestedObjectTypeOf[T]) ValueFromList(ctx context.Context, listval basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
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

	return listNestedObjectValueOf[T]{ListValue: v}, diags
}

func newSetNestedObjectTypeOf[T any](ctx context.Context, elemType attr.Type) setNestedObjectTypeOf[T] {
	return setNestedObjectTypeOf[T]{basetypes.SetType{ElemType: elemType}}
}

func NewListNestedObjectValueOfNull[T any](ctx context.Context) listNestedObjectValueOf[T] {
	return listNestedObjectValueOf[T]{ListValue: basetypes.NewListNull(NewObjectTypeOf[T](ctx))}
}

func NewListNestedObjectValueOfUnknown[T any](ctx context.Context) listNestedObjectValueOf[T] {
	return listNestedObjectValueOf[T]{ListValue: basetypes.NewListUnknown(NewObjectTypeOf[T](ctx))}
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

func newListNestedObjectTypeOf[T any](ctx context.Context) listNestedObjectTypeOf[T] {
	return listNestedObjectTypeOf[T]{basetypes.ListType{ElemType: NewObjectTypeOf[T](ctx)}}
}
