package providerbuilder

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
						CustomType: newSetNestedObjectTypeOf[promptOverrideConfigurationModel](context.Background()),
						Optional:   true,
						Computed:   true,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
						Validators: []validator.List{
							listvalidator.SizeAtMost(1),
						},
						//ElementType: types.ObjectType{
						//	AttrTypes: fwtypes.AttributeTypesMust[promptOverrideConfigurationModel](ctx),
						//},
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

type setNestedObjectTypeOf[T any] struct {
	basetypes.SetType
}

func newSetNestedObjectTypeOf[T any](ctx context.Context) setNestedObjectTypeOf[T] {
	return setNestedObjectTypeOf[T]{basetypes.SetType{ElemType: types.ObjectType{}[promptConfigurationModel]}}
}

//func NewListNestedObjectTypeOf[T any](ctx context.Context) listNestedObjectTypeOf[T] {
//	return listNestedObjectTypeOf[T]{basetypes.ListType{ElemType: NewObjectTypeOf[T](ctx)}}
//}
