package tfbridgetests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
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

	pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
}

type modifyValuePlanModifier struct {
	planmodifier.String
}

func (c modifyValuePlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	resp.PlanValue = basetypes.NewStringValue("Modified val")
}

func (c modifyValuePlanModifier) Description(context.Context) string {
	return "Modify value plan modifier"
}

func (c modifyValuePlanModifier) MarkdownDescription(context.Context) string {
	return "Modify value plan modifier"
}

func TestPlanModifiers(t *testing.T) {
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
							PlanModifiers: []planmodifier.String{
								modifyValuePlanModifier{},
							},
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

	require.Equal(t, "Modified val", upRes.Outputs["changeReason"].Value)

	pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
}

func TestDefaultAndPlanModifier(t *testing.T) {
	// Note plan modifiers trump defaults!
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
							PlanModifiers: []planmodifier.String{
								modifyValuePlanModifier{},
							},
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

	require.Equal(t, "Modified val", upRes.Outputs["changeReason"].Value)
}

func TestComputedDefaultIgnoreChanges(t *testing.T) {
	replaceProgram := func(t *testing.T, pt *pulumitest.PulumiTest, program string) {
		program = strings.ReplaceAll(program, "\t", "    ")
		pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
		err := os.WriteFile(pulumiYamlPath, []byte(program), 0o600)
		require.NoError(t, err)
	}
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
							Computed: true,
							Default:  stringdefault.StaticString("Default val"),
						},
					},
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	defaultProgram := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
outputs:
    otherProp: ${mainRes.otherProp}`

	explicitProgram := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            otherProp: %s
        options:
            ignoreChanges: ["otherProp"]
outputs:
    otherProp: ${mainRes.otherProp}`

	explicit1 := fmt.Sprintf(explicitProgram, "val1")
	explicit2 := fmt.Sprintf(explicitProgram, "val2")

	runTest := func(t *testing.T, program1, program2, expected1 string) {
		pt := pulCheck(t, prov, program1)
		upRes := pt.Up()
		t.Logf(upRes.StdOut)

		require.Equal(t, expected1, upRes.Outputs["otherProp"].Value)

		replaceProgram(t, pt, program2)
		res := pt.Up(optup.ExpectNoChanges(), optup.Diff())
		t.Logf(res.StdOut)
		require.Equal(t, expected1, upRes.Outputs["otherProp"].Value)
	}

	t.Run("Explicit to explicit", func(t *testing.T) {
		runTest(t, explicit1, explicit2, "val1")
	})

	t.Run("Default to explicit", func(t *testing.T) {
		runTest(t, defaultProgram, explicit1, "Default val")
	})

	t.Run("Explicit to default", func(t *testing.T) {
		runTest(t, explicit1, defaultProgram, "val1")
	})
}
