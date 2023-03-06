// Copyright 2016-2023, Pulumi Corporation.
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

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// We are testing that there are no panics when checking valid resources, even when the
// resource is missing in the schema.
func TestNameCheckMissingResource(t *testing.T) {
	p := testprovider.ProviderMiniRandom()
	info := tfbridge.ProviderInfo{
		P: shim.NewProvider(p),
	}

	properties := func(names ...string) map[string]pschema.PropertySpec {
		props := make(map[string]pschema.PropertySpec)
		for _, name := range names {
			props[name] = pschema.PropertySpec{}
		}
		return props
	}

	randomIntegerProps := properties("keepers", "min",
		"max", "seed", "result")

	for _, resources := range []map[string]pschema.ResourceSpec{
		{},
		{
			"random/index:integer:Integer": {
				InputProperties: randomIntegerProps,
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Properties: randomIntegerProps,
				}},
		},
	} {
		schema := pschema.PackageSpec{
			Name:      "random",
			Resources: resources,
		}
		err := nameCheck(info, schema, newRenamesBuilder("random", "random_"), NoErrorSink(t))
		assert.NoError(t, err)
	}
}

func NoErrorSink(t *testing.T) diag.Sink {
	e := errOnWrite{t}
	return diag.DefaultSink(e, e, diag.FormatOptions{
		Color: colors.Never,
	})
}

type errOnWrite struct{ t *testing.T }

func (e errOnWrite) Write(p []byte) (int, error) {
	e.t.Fatalf("Attempted to write %s", string(p))
	return 0, nil
}
