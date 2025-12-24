// Copyright 2025, Pulumi Corporation.
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

package tfgen

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestCountStats(t *testing.T) {
	t.Parallel()

	testSchema := schema.PackageSpec{
		Functions: map[string]schema.FunctionSpec{
			"test:index/getFoo:getFoo": {
				Description: "0123456789",
				Inputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"input1": {
							Description: "0123456789",
						},
						"input2": {
							Description: "0",
						},
						"inputMissingDesc1": {},
						"inputMissingDesc2": {},
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"output1": {
							Description: "0123456789",
						},
						"output2": {
							Description: "01",
						},
						"outputMissingDesc1": {},
						"outputMissingDesc2": {},
						"outputMissingDesc3": {},
					},
				},
			},
			"test:index/getBar:getBar": {
				Description: "0",
				Inputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"input1": {
							Description: "0",
						},
						"inputMissingDesc3": {},
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"output1": {
							Description: "0",
						},
						"outputMissingDesc4": {},
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"test:index/foo:Foo": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "0123456789",
					Properties: map[string]schema.PropertySpec{
						"output1": {
							Description: "0123456789",
						},
						"output2": {
							Description: "01234",
						},
						"noDesc1": {
							Description: "",
						},
						"noDesc2": {
							Description: "",
						},
						"noDesc3": {
							Description: "",
						},
						"noDesc4": {
							Description: "",
						},
						"noDesc5": {
							Description: "",
						},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"noDesc1": {
						Description: "",
					},
					"noDesc2": {
						Description: "",
					},
					"hasDesc": {
						Description: "What it does",
					},
				},
			},
		},
	}

	stats := countStats(testSchema)

	// The example schema above is designed to give a unique number for each property as much as possible to provide
	// the highest-confidence test results.
	assert.Equal(t, 1, stats.Resources.TotalResources)
	assert.Equal(t, 10, stats.Resources.TotalDescriptionBytes)

	assert.Equal(t, 3, stats.Resources.TotalInputProperties)
	assert.Equal(t, 2, stats.Resources.InputPropertiesMissingDescriptions)

	assert.Equal(t, 7, stats.Resources.TotalOutputProperties)
	assert.Equal(t, 5, stats.Resources.OutputPropertiesMissingDescriptions)

	assert.Equal(t, 2, stats.Functions.TotalFunctions)
	assert.Equal(t, 11, stats.Functions.TotalDescriptionBytes)

	assert.Equal(t, 3, stats.Functions.InputPropertiesMissingDescriptions)
	assert.Equal(t, 12, stats.Functions.TotalInputPropertyDescriptionBytes)

	assert.Equal(t, 4, stats.Functions.OutputPropertiesMissingDescriptions)
	assert.Equal(t, 13, stats.Functions.TotalOutputPropertyDescriptionBytes)
}

func TestCountStats_InternalRefs_Inputs(t *testing.T) {
	t.Parallel()

	testSchema := schema.PackageSpec{
		Types: map[string]schema.ComplexTypeSpec{
			"test:index/myType:myType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"myProperty": {
							TypeSpec: schema.TypeSpec{
								Type: "integer",
							},
						},
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"test:index/myResource:/myResource": {
				InputProperties: map[string]schema.PropertySpec{
					"myProperty": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:index/myType:myType",
						},
					},
					"myProperty2": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:index/myType:myType",
						},
					},
				},
			},
		},
	}

	stats := countStats(testSchema)

	assert.Equal(t, 3, stats.Resources.TotalInputProperties)
}

func TestCountStats_InternalRefs_Outputs(t *testing.T) {
	t.Parallel()

	testSchema := schema.PackageSpec{
		Types: map[string]schema.ComplexTypeSpec{
			"test:index/myType:myType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"myProperty": {
							TypeSpec: schema.TypeSpec{
								Type: "integer",
							},
						},
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"test:index/myResource:/myResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "0123456789",
					Properties: map[string]schema.PropertySpec{
						"myProperty": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/test:index/myType:myType",
							},
						},
						"myProperty2": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/test:index/myType:myType",
							},
						},
					},
				},
			},
		},
	}

	stats := countStats(testSchema)

	assert.Equal(t, 3, stats.Resources.TotalOutputProperties)
}

func TestCountStats_NoInputs(t *testing.T) {
	t.Parallel()

	testSchema := schema.PackageSpec{
		Functions: map[string]schema.FunctionSpec{
			"test:index/getFooNoInputs:getFooNoInputs": {
				Description: "This is a function that has no inputs, like getGlobalClient on the auth0 provider.",
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"output1": {
							Description: "0123456789",
						},
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"test:index/noInputs:noInputs": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "This is a resource that has no inputs. " +
						"Not known whether this case exists in the wild, but it sure shouldn't panic if we hit it!",
					Properties: map[string]schema.PropertySpec{
						"output1": {
							Description: "0123456789",
						},
					},
				},
			},
		},
	}

	_ = countStats(testSchema)
}

func TestCountStats_ExternalRef(t *testing.T) {
	t.Parallel()

	testSchema := schema.PackageSpec{
		Resources: map[string]schema.ResourceSpec{
			"awsx:cloudtrail:Trail": {
				InputProperties: map[string]schema.PropertySpec{
					"bucket": {
						TypeSpec: schema.TypeSpec{
							Ref: "/aws/v5.4.0/schema.json#/resources/aws:s3%2Fbucket:Bucket",
						},
						Description: "The managed S3 Bucket where the Trail will place its logs.",
					},
				},
			},
		},
	}

	stats := countStats(testSchema)

	// We are mostly testing that we did not get a panic because of the external type ref
	assert.Equal(t, stats.Resources.TotalResources, 1)
	assert.Equal(t, stats.Resources.TotalInputProperties, 1)
}

func TestVersionlessName(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"config:assumeRoleWithWebIdentity",
		versionlessName("#/types/aws:config/assumeRoleWithWebIdentity:assumeRoleWithWebIdentity"),
	)
}
