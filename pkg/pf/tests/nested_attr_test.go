// Copyright 2016-2022, Pulumi Corporation.
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

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

func TestNestedType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	info := testprovider.SyntheticTestBridgeProvider()
	res, err := info.P.(pf.ShimProvider).Resources(ctx)
	require.NoError(t, err)
	testresTypeName := runtypes.TypeOrRenamedEntityName("testbridge_testres")
	testresType := res.Schema(testresTypeName).Type(ctx)

	obj := testresType.(tftypes.Object).AttributeTypes["services"].(tftypes.List).ElementType.(tftypes.Object)
	assert.True(t, obj.AttributeTypes["protocol"].Is(tftypes.String))
}
