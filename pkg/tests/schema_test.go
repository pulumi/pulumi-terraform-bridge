package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedFullyComputed(t *testing.T) {
	t.Parallel()
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"testprovider_res": {
				Schema: map[string]*schema.Schema{
					"list_block": {
						Type:     schema.TypeList,
						Required: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"a1": {
									Type:     schema.TypeString,
									Computed: true,
								},
								"a2": {
									Type:     schema.TypeString,
									Optional: true,
								},
								"a3": {
									Type:     schema.TypeString,
									Required: true,
								},
							},
						},
					},
					"object_block": {
						Type:     schema.TypeList,
						Optional: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"b1": {
									Type:     schema.TypeString,
									Computed: true,
								},
								"b2": {
									Type:     schema.TypeString,
									Optional: true,
								},
								"b3": {
									Type:     schema.TypeString,
									Required: true,
								},
							},
						},
					},
				},
			},
		},
	}

	info := pulcheck.BridgedProvider(t, "testprovider", p)

	schema, err := tfgen.GenerateSchema(info, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)

	marshal := func(s pschema.PackageSpec, w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	toString := func(s pschema.PackageSpec) string {
		buf := bytes.Buffer{}
		err := marshal(s, &buf)
		assert.NoError(t, err)
		return buf.String()
	}

	autogold.ExpectFile(t, autogold.Raw(toString(schema)))
}
