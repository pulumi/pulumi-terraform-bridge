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
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestCachingPluginHost(t *testing.T) {
	h := &testHost{nil, false}
	c := newCachingProviderHost(h)

	v1 := semver.MustParse("1.0.0")
	v2 := semver.MustParse("0.0.1-alpha")

	for _, pkg := range []tokens.Package{"a", "b"} {
		for _, version := range []*semver.Version{nil, &v1, &v2} {

			p1, err := h.Provider(pkg, version)
			require.NoError(t, err)

			p2, err := c.Provider(pkg, version)
			require.NoError(t, err)

			require.Equal(t, p1.(*testProvider).pkg, p2.(*testProvider).pkg)
			require.Equal(t, p1.(*testProvider).version, p2.(*testProvider).version)
		}
	}

	_, err := newCachingProviderHost(&testHost{nil, true}).Provider("a", &v1)
	require.Error(t, err)
}

type testProvider struct {
	plugin.Provider
	pkg     tokens.Package
	version *semver.Version
}

type testHost struct {
	plugin.Host
	fail bool
}

func (th *testHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	if th.fail {
		return nil, fmt.Errorf("failed")
	}
	return &testProvider{nil, pkg, version}, nil
}
