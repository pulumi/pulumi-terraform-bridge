// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testprovider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	cschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func ProviderMiniTalos() tfbridge.ProviderInfo {
	const (
		talosPkg   = "talos"
		machineMod = "machine"
	)

	schemaProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"talos_machine_secrets": {
				Schema: map[string]*schema.Schema{
					"machine_secrets": {
						Type: schema.TypeMap,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{},
						},
					},
				},
			},
		},
	}

	info := tfbridge.ProviderInfo{
		P:    shimv2.NewProvider(schemaProvider),
		Name: "talos",
		Resources: map[string]*tfbridge.ResourceInfo{
			"talos_machine_secrets": {
				Tok: tfbridge.MakeResource(talosPkg, machineMod, "Secrets"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"machine_secrets": {
						Elem: &tfbridge.SchemaInfo{
							Type: "talos:machine/generated:MachineSecrets",
						},
					},
				},
			},
		},
		ExtraTypes: map[string]cschema.ComplexTypeSpec{
			"talos:machine/generated:MachineSecrets": {
				ObjectTypeSpec: cschema.ObjectTypeSpec{
					Type: "object",
				},
			},
		},
	}

	return info
}
