package tfbridgetests

import (
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name:           "test_res",
				ResourceSchema: rschema.Schema{},
			},
		},
	}

	prov := BridgedProvider(t, &provBuilder)

	program := ``

	PulCheck(t, prov, program)
}
