package adapter

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestReadTerraformState(t *testing.T) {
	state, err := readTerraformState("testdata/tfstate.json")
	if err != nil {
		t.Fatalf("failed to read Terraform state: %v", err)
	}

	autogold.ExpectFile(t, state)
}

func TestResourceToCtyValue(t *testing.T) {
	state, err := readTerraformState("testdata/tfstate.json")
	if err != nil {
		t.Fatalf("failed to read Terraform resource: %v", err)
	}

	res := state.Resources[1]
	require.Equal(t, "aws_apigatewayv2_api", res.TypeName)

	resourceType := valueshim.FromCtyType(cty.Object(
		map[string]cty.Type{
			"api_endpoint":                 cty.String,
			"api_key_selection_expression": cty.String,
			"arn":                          cty.String,
			"body":                         cty.String,
			"cors_configuration": cty.List(cty.Object(map[string]cty.Type{
				"allow_origins":  cty.List(cty.String),
				"allow_methods":  cty.List(cty.String),
				"allow_headers":  cty.List(cty.String),
				"expose_headers": cty.List(cty.String),
				"max_age":        cty.Number,
			})),
			"credentials_arn":              cty.String,
			"description":                  cty.String,
			"disable_execute_api_endpoint": cty.Bool,
			"execution_arn":                cty.String,
			"fail_on_warnings":             cty.Bool,
			"id":                           cty.String,
			"ip_address_type":              cty.String,
			"name":                         cty.String,
			"protocol_type":                cty.String,
			"region":                       cty.String,
			"route_key":                    cty.String,
			"route_selection_expression":   cty.String,
			"tags":                         cty.Map(cty.String),
			"tags_all":                     cty.Map(cty.String),
			"target":                       cty.String,
			"version":                      cty.String,
		},
	))

	value, err := resourceToCtyValue(&res, resourceType)

	autogold.ExpectFile(t, value)
}
