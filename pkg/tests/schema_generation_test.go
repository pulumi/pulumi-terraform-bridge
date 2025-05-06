package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

func marshalPackageSpec(spec pschema.PackageSpec) (string, error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(spec)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestRequiredInputWithDefault(t *testing.T) {
	t.Parallel()

	p := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"testprovider_res": {
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
						DefaultFunc: func() (interface{}, error) {
							return "default", nil
						},
					},
					"req": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
	}

	provider := pulcheck.BridgedProvider(t, "testprovider", p)

	schema, err := tfgen.GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)
	require.Empty(t, schema.Resources["testprovider:index:Res"].RequiredInputs)
	spec, err := marshalPackageSpec(schema)
	require.NoError(t, err)
	autogold.ExpectFile(t, autogold.Raw(spec))

	resourceSchema := schema.Resources["testprovider:index/res:Res"]
	require.NotContains(t, resourceSchema.RequiredInputs, "name")
}
