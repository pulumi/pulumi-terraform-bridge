package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            s: "hello"`

	pt := pulCheck(t, prov, program)

	pt.Up()
}

func TestDefaults(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"other_prop": rschema.StringAttribute{
							Optional: true,
						},
						"change_reason": rschema.StringAttribute{
							Optional: true,
							// I've been unable to find an example of a non-Computed resource with a default value in the wild.
							// Nothing in the docs or validation prohibits this.
							Computed: true,
							Default:  stringdefault.StaticString("Default val"),
						},
					},
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            otherProp: "val"
outputs:
    changeReason: ${mainRes.changeReason}`

	pt := pulCheck(t, prov, program)
	upRes := pt.Up()
	t.Logf(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(optpreview.Diff())

	refreshRes := pt.Refresh(optrefresh.ExpectNoChanges())
	t.Logf(refreshRes.StdOut)

	pt.Destroy()

	res := pt.Import("prov:index/test:Test", "mainRes", "new-id", "")
	t.Logf(res.Stdout)

	prevRes := pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
	t.Logf(prevRes.StdOut)
}

func TestImportAndRefreshWithDefault(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"other_prop": rschema.StringAttribute{
							Optional: true,
						},
						"change_reason": rschema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString("Default val"),
						},
					},
				},
				ReadFunc: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
					resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
					resp.State.SetAttribute(ctx, path.Root("other_prop"), "val")
					resp.State.SetAttribute(ctx, path.Root("change_reason"), "Default val")
				},
				ImportStateFunc: func(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
					resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
					resp.State.SetAttribute(ctx, path.Root("other_prop"), "val")
					resp.State.SetAttribute(ctx, path.Root("change_reason"), "Default val")
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            otherProp: "val"
outputs:
    changeReason: ${mainRes.changeReason}`

	pt := pulCheck(t, prov, program)
	upRes := pt.Up()
	t.Logf(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(optpreview.Diff())

	refreshRes := pt.Refresh(optrefresh.ExpectNoChanges())
	t.Logf(refreshRes.StdOut)

	pt.Destroy()

	res := pt.Import("prov:index/test:Test", "mainRes", "new-id", "")
	t.Logf(res.Stdout)

	prevRes := pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
	t.Logf(prevRes.StdOut)
}
