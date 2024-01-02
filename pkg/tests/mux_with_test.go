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

package tests

import (
	"context"
	"testing"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func todo[T any]() T { panic("TODO") }

func TestMuxWithProvider(t *testing.T) {
	ctx := context.Background()

	prov := todo[tfbridge.ProviderInfo]()
	pulumiSchema := todo[[]byte]()
	grpcTestCase := todo[string]()

	server := tfbridge.NewProvider(ctx, nil, prov.Name, "1.0.0", prov.P, prov, pulumiSchema)

	testutils.Replay(t, server, grpcTestCase)
}
