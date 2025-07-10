package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

// The test is set up to reproduce https://github.com/pulumi/pulumi-vsphere/issues/824
func Test_RegressVSphere824(t *testing.T) {
	t.Parallel()

	subResourceSchema := map[string]*schema.Schema{
		"label": {
			Type:     schema.TypeString,
			Required: true,
		},
		"datastore_id": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"disk": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Description: "A specification for a virtual disk device on this virtual machine.",
				MaxItems:    60,
				Elem:        &schema.Resource{Schema: subResourceSchema},
			},
		},
	}

	tfp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{"prov_test": res},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	program1 := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      disks:
        - label: label1
          datastoreId: ds1
        - label: label2
          datastoreId: ds2
    options:
      ignoreChanges:
        - "disks[*].datastoreId"
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program1)
	out := pt.Up(t)
	t.Logf("# update 1: %v", out.StdErr+out.StdOut)

	d := pt.ExportStack(t)
	text, err := json.MarshalIndent(d, "", "  ")
	require.NoError(t, err)
	t.Logf("STATE: %s", text)

	program2 := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      disks:
        - label: label1
          datastoreId: ds1
        - label: label2
          datastoreId: ds2
        - label: label3
          datastoreId: ds3
    options:
      ignoreChanges:
        - "disks[*].datastoreId"
`

	err = os.WriteFile(filepath.Join(pt.WorkingDir(), "Pulumi.yaml"), []byte(program2), 0655)
	require.NoError(t, err)

	out2 := pt.Up(t)
	t.Logf("# update 2: %v", out2.StdErr+out2.StdOut)
}
