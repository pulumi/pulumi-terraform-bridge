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

package tfbridgetests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestGetMapping(t *testing.T) {
	ctx := context.Background()
	info := testprovider.RandomProvider()

	p, err := tfbridge.NewProvider(ctx, info, genMetadata(t, info))
	assert.NoError(t, err)

	{
		m, p, err := p.GetMappingWithContext(ctx, "unknown-key")
		assert.NoError(t, err)
		assert.Empty(t, m)
		assert.Empty(t, p)
	}

	for _, key := range []string{"tf", "terraform"} {
		m, p, err := p.GetMappingWithContext(ctx, key)
		assert.NoError(t, err)

		assert.Equal(t, "random", p)

		var info tfbridge0.MarshallableProviderInfo
		err = json.Unmarshal(m, &info)
		assert.NoError(t, err)

		assert.Equal(t, "random", info.Name)
		assert.Contains(t, info.Resources, "random_integer")
		assert.Equal(t, "random:index/randomInteger:RandomInteger", string(info.Resources["random_integer"].Tok))
	}
}
