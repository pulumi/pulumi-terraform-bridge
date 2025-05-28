package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
)

func TestCreateCustomTimeoutsCrossTest(t *testing.T) {
	t.Parallel()
	test := func(
		t *testing.T,
		schemaCreateTimeout *time.Duration,
		programTimeout *string,
		expected time.Duration,
		ExpectFail bool,
	) {
		var pulumiCapturedTimeout *time.Duration
		var tfCapturedTimeout *time.Duration
		prov := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"prov_test": {
					Schema: map[string]*schema.Schema{
						"prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
						t := rd.Timeout(schema.TimeoutCreate)
						if pulumiCapturedTimeout == nil {
							pulumiCapturedTimeout = &t
						} else {
							tfCapturedTimeout = &t
						}
						rd.SetId("id")
						return diag.Diagnostics{}
					},
					Timeouts: &schema.ResourceTimeout{
						Create: schemaCreateTimeout,
					},
				},
			},
		}

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)
		pulumiTimeout := `""`
		if programTimeout != nil {
			pulumiTimeout = fmt.Sprintf(`"%s"`, *programTimeout)
		}

		tfTimeout := "null"
		if programTimeout != nil {
			tfTimeout = fmt.Sprintf(`"%s"`, *programTimeout)
		}

		program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
	mainRes:
		type: prov:Test
		properties:
			prop: "val"
		options:
			customTimeouts:
				create: %s
`, pulumiTimeout)

		pt := pulcheck.PulCheck(t, bridgedProvider, program)
		pt.Up(t)
		// We pass custom timeouts in the program if the resource does not support them.

		require.NotNil(t, pulumiCapturedTimeout)
		require.Nil(t, tfCapturedTimeout)

		tfProgram := fmt.Sprintf(`
resource "prov_test" "mainRes" {
	prop = "val"
	timeouts {
		create = %s
	}
}`, tfTimeout)

		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfcheck.NewTFDriverOpts{
			SDKProvider: prov,
		})
		tfdriver.Write(t, tfProgram)

		plan, err := tfdriver.Plan(t)
		if ExpectFail {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		err = tfdriver.ApplyPlan(t, plan)
		require.NoError(t, err)
		require.NotNil(t, tfCapturedTimeout)

		assert.Equal(t, *pulumiCapturedTimeout, *tfCapturedTimeout)
		assert.Equal(t, *pulumiCapturedTimeout, expected)
	}

	oneSecString := "1s"
	one := 1 * time.Second
	// twoSecString := "2s"
	two := 2 * time.Second

	tests := []struct {
		name                string
		schemaCreateTimeout *time.Duration
		programTimeout      *string
		expected            time.Duration
		expectFail          bool
	}{
		{
			"schema specified timeout",
			&one,
			nil,
			one,
			false,
		},
		{
			"program specified timeout",
			&two,
			&oneSecString,
			one,
			false,
		},
		{
			"program specified without schema timeout",
			nil,
			&oneSecString,
			one,
			true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			test(t, tc.schemaCreateTimeout, tc.programTimeout, tc.expected, tc.expectFail)
		})
	}
}
