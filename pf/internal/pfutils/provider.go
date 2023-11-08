package pfutils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Creates a pf provider that's backed by a protov6.ProviderServer
func SchemaOnlyProvider(name, version string, server tfprotov6.ProviderServer) provider.Provider {
	return &schemaOnlyProvider{name: name, version: version, server: server}
}

type schemaOnlyProvider struct {
	name    string // The TF name of the provider, as used for TF name prefixes.
	version string // The version of the provider (not currently hooked up upstream)
	server  tfprotov6.ProviderServer

	// A cached schema from the wire, may be nil
	schema *tfprotov6.GetProviderSchemaResponse
}

var _ provider.Provider = &schemaOnlyProvider{}

func (p schemaOnlyProvider) Metadata(ctx context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = p.name
	resp.Version = p.version
}

func (p *schemaOnlyProvider) Schema(ctx context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	schema, err := p.getSchema(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Schema", err.Error())
		return
	}
	resp.Schema, resp.Diagnostics = convertProviderSchema(schema.Provider)
}

func (p *schemaOnlyProvider) getSchema(ctx context.Context) (*tfprotov6.GetProviderSchemaResponse, error) {
	if p.schema != nil {
		return p.schema, nil
	}
	var err error
	p.schema, err = p.server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	return p.schema, err
}

func (schemaOnlyProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
	contract.Failf("Configure is not implemented on schemaOnlyProvider")
}

func (p *schemaOnlyProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	schema, err := p.getSchema(ctx)
	if err != nil {
		// Since there is no method to return an error here, we panic.
		panic(err.Error())
	}
	resp := make([]func() datasource.DataSource, 0, len(schema.DataSourceSchemas))
	for k, v := range schema.DataSourceSchemas {
		resp = append(resp, schemaOnlyDataSource{name: k, schema: v}.identity)
	}
	return resp
}

func (p *schemaOnlyProvider) Resources(ctx context.Context) []func() resource.Resource {
	schema, err := p.getSchema(ctx)
	if err != nil {
		// Since there is no method to return an error here, we panic.
		panic(err.Error())
	}
	resp := make([]func() resource.Resource, 0, len(schema.ResourceSchemas))
	for k, v := range schema.ResourceSchemas {
		resp = append(resp, schemaOnlyResource{name: k, schema: v}.identity)
	}
	return resp
}

func convertProviderSchema(req *tfprotov6.Schema) (pschema.Schema, diag.Diagnostics) {
	var schema pschema.Schema

	schema.MarkdownDescription, schema.Description = getDescription(
		req.Block.DescriptionKind, req.Block.Description)

	if req.Block.Deprecated {
		schema.DeprecationMessage = "This provider is deprecated"
	}

	attrs := make(map[string]pschema.Attribute)
	for k, v := range convertDataSourceAttributes(req.Block.Attributes) {
		attrs[k] = v.(pschema.Attribute) // This might be a VERY BAD IDEA, but it compiles
	}
	blocks := make(map[string]pschema.Block)
	for k, v := range convertDataSourceNestedBlock(req.Block.BlockTypes) {
		blocks[k] = v.(pschema.Block)
	}
	schema.Attributes = attrs
	schema.Blocks = blocks

	return schema, nil
}

func convertResourceSchema(req *tfprotov6.Schema) (rschema.Schema, diag.Diagnostics) {
	var schema rschema.Schema
	schema.MarkdownDescription, schema.Description = getDescription(
		req.Block.DescriptionKind, req.Block.Description)

	if req.Block.Deprecated {
		schema.DeprecationMessage = "This resource is deprecated"
	}

	attrs := make(map[string]rschema.Attribute)
	for k, v := range convertDataSourceAttributes(req.Block.Attributes) {
		attrs[k] = v.(rschema.Attribute) // This might be a VERY BAD IDEA, but it compiles
	}
	blocks := make(map[string]rschema.Block)
	for k, v := range convertDataSourceNestedBlock(req.Block.BlockTypes) {
		blocks[k] = v.(rschema.Block)
	}
	schema.Attributes = attrs
	schema.Blocks = blocks

	return schema, nil
}

func convertDataSourceSchema(req *tfprotov6.Schema) (dschema.Schema, diag.Diagnostics) {
	var schema dschema.Schema
	schema.MarkdownDescription, schema.Description = getDescription(
		req.Block.DescriptionKind, req.Block.Description)

	if req.Block.Deprecated {
		schema.DeprecationMessage = "This data source is deprecated"
	}

	schema.Attributes = convertDataSourceAttributes(req.Block.Attributes)
	schema.Blocks = convertDataSourceNestedBlock(req.Block.BlockTypes)

	return schema, nil
}

func getDescription(kind tfprotov6.StringKind, rawDesc string) (mdDesc, desc string) {
	switch kind {
	case tfprotov6.StringKindPlain:
		return "", rawDesc
	case tfprotov6.StringKindMarkdown:
		return rawDesc, ""
	default: // Unknown kind
		return rawDesc, rawDesc
	}

}

func convertDataSourceAttributes(attributes []*tfprotov6.SchemaAttribute) map[string]dschema.Attribute {
	attrs := make(map[string]dschema.Attribute, len(attributes))
	for _, a := range attributes {

		if _, duplicate := attrs[a.Name]; duplicate {
			contract.Failf("Duplicate attribute: %q", a.Name)
		}

		if a.NestedType != nil {
			panic("NOT IMPLEMENTED YET")
			continue
		}

		mdDesc, desc := getDescription(a.DescriptionKind, a.Description)
		switch {
		case a.Type.Is(tftypes.Bool):
			attrs[a.Name] = dschema.BoolAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
			}
		case a.Type.Is(tftypes.String):
			attrs[a.Name] = dschema.StringAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
			}
		case a.Type.Is(tftypes.Number):
			attrs[a.Name] = dschema.NumberAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
			}
		case a.Type.Is(tftypes.Map{}):
			attrs[a.Name] = dschema.MapAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
				ElementType:         convertTftypeToAttrType(a.Type.(tftypes.Map).ElementType),
			}
		case a.Type.Is(tftypes.Set{}):
			attrs[a.Name] = dschema.MapAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
				ElementType:         convertTftypeToAttrType(a.Type.(tftypes.Set).ElementType),
			}
		case a.Type.Is(tftypes.List{}):
			attrs[a.Name] = dschema.MapAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
				ElementType:         convertTftypeToAttrType(a.Type.(tftypes.List).ElementType),
			}
		case a.Type.Is(tftypes.Object{}):
			objAttrs := make(map[string]attr.Type, len(a.Type.(tftypes.Object).AttributeTypes))
			for k, objA := range a.Type.(tftypes.Object).AttributeTypes {
				objAttrs[k] = convertTftypeToAttrType(objA)
			}
			attrs[a.Name] = dschema.ObjectAttribute{
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Description:         desc,
				MarkdownDescription: mdDesc,
				AttributeTypes:      objAttrs,
			}
		default:
			contract.Failf("UNKNOWN TYPE: %s", a.Type.String())
		}
	}
	return attrs
}

func convertTftypeToAttrType(ty tftypes.Type) attr.Type {
	switch {
	case ty.Is(tftypes.Bool):
		return types.BoolType
	case ty.Is(tftypes.String):
		return types.StringType
	case ty.Is(tftypes.Number):
		return types.NumberType
	default:
		return attrTypeShim{ty}
	}
}

type attrTypeShim struct{ tftypes.Type }

func (a attrTypeShim) TerraformType(context.Context) tftypes.Type {
	return a.Type
}

func (attrTypeShim) ValueFromTerraform(context.Context, tftypes.Value) (attr.Value, error) {
	contract.Failf("attrTypeShim does not implement ValueFromTerraform")
	return nil, nil
}
func (attrTypeShim) ValueType(context.Context) attr.Value {
	contract.Failf("attrTypeShim does not implement ValueType")
	return nil
}
func (a attrTypeShim) Equal(other attr.Type) bool {
	b, ok := other.(attrTypeShim)
	if ok {
		return a.Type.Equal(b.Type)
	}
	return false
}

func convertDataSourceNestedBlock(protoBlocks []*tfprotov6.SchemaNestedBlock) map[string]dschema.Block {
	blocks := make(map[string]dschema.Block, len(protoBlocks))
	for _, protoBlock := range protoBlocks {
		if _, duplicate := blocks[protoBlock.TypeName]; duplicate {
			contract.Failf("Duplicate block: " + protoBlock.TypeName)
		}

		mdDesc, desc := getDescription(protoBlock.Block.DescriptionKind, protoBlock.Block.Description)

		switch protoBlock.Nesting {
		case tfprotov6.SchemaNestedBlockNestingModeList:
			blocks[protoBlock.TypeName] = dschema.ListNestedBlock{
				MarkdownDescription: mdDesc,
				Description:         desc,
				NestedObject:        convertDataSourceNestedBlockObject(protoBlock.Block),
			}
		case tfprotov6.SchemaNestedBlockNestingModeSet:
			blocks[protoBlock.TypeName] = dschema.SetNestedBlock{
				MarkdownDescription: mdDesc,
				Description:         desc,
				NestedObject:        convertDataSourceNestedBlockObject(protoBlock.Block),
			}
		case tfprotov6.SchemaNestedBlockNestingModeSingle:
			obj := convertDataSourceNestedBlockObject(protoBlock.Block)
			blocks[protoBlock.TypeName] = dschema.SingleNestedBlock{
				MarkdownDescription: mdDesc,
				Description:         desc,
				Attributes:          obj.Attributes,
				Blocks:              obj.Blocks,
			}
		default:
			panic("Unknown block nesting: " + protoBlock.Nesting.String())
		}
	}

	return blocks
}

func convertDataSourceNestedBlockObject(block *tfprotov6.SchemaBlock) dschema.NestedBlockObject {
	return dschema.NestedBlockObject{
		Attributes: convertDataSourceAttributes(block.Attributes),
		Blocks:     convertDataSourceNestedBlock(block.BlockTypes),
	}
}

type schemaOnlyDataSource struct {
	name   string
	schema *tfprotov6.Schema
}

func (s schemaOnlyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + s.name
}

// Schema should return the schema for this data source.
func (s schemaOnlyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema, resp.Diagnostics = convertDataSourceSchema(s.schema)
}

// Read is called when the provider must read data source values in
// order to update state. Config values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (s schemaOnlyDataSource) Read(context.Context, datasource.ReadRequest, *datasource.ReadResponse) {
	contract.Failf("schemaOnlyDataSource does not implement Read")
}

func (s schemaOnlyDataSource) identity() datasource.DataSource { return s }

type schemaOnlyResource struct {
	name   string
	schema *tfprotov6.Schema
}

func (s schemaOnlyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + s.name
}

// Schema should return the schema for this resource.
func (s schemaOnlyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema, resp.Diagnostics = convertResourceSchema(s.schema)
}

// Create is called when the provider must create a new resource. Config
// and planned state values should be read from the
// CreateRequest and new state values set on the CreateResponse.
func (s schemaOnlyResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
	contract.Failf("schemaOnlyResource does not implement Create")
}

// Read is called when the provider must read resource values in order
// to update state. Planned state values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (s schemaOnlyResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
	contract.Failf("schemaOnlyResource does not implement Read")
}

// Update is called to update the state of the resource. Config, planned
// state, and prior state values should be read from the
// UpdateRequest and new state values set on the UpdateResponse.
func (s schemaOnlyResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	contract.Failf("schemaOnlyResource does not implement Update")
}

// Delete is called when the provider must delete the resource. Config
// values may be read from the DeleteRequest.
//
// If execution completes without error, the framework will automatically
// call DeleteResponse.State.RemoveResource(), so it can be omitted
// from provider logic.
func (s schemaOnlyResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	contract.Failf("schemaOnlyResource does not implement Delete")
}

func (s schemaOnlyResource) identity() resource.Resource { return s }
