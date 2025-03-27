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

package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

// RandomProvider returns a [integration.LocalDependency] reference to the random provider
// installed via `make install_plugins`.
func RandomProvider(t *testing.T) integration.LocalDependency {
	return pluginDependency(t, "random", semver.Version{Major: 4, Minor: 16, Patch: 3})
}

func pluginDependency(t *testing.T, name string, version semver.Version) integration.LocalDependency {

	pluginSpec, err := workspace.NewPluginSpec(name, apitype.ResourcePlugin, &version, "", nil)
	require.NoError(t, err)
	path, err := workspace.GetPluginPath(
		diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		}),
		pluginSpec, nil)
	require.NoError(t, err,
		`The %s provider at this version should have been installed by "make install_plugins"`, name)
	return integration.LocalDependency{
		Package: name,
		Path:    filepath.Dir(path),
	}
}
