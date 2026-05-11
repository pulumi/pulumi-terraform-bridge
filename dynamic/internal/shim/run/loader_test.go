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

package run

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noBackoff replaces etxtbsyBackoff with a zero-sleep variant so retry tests
// run in microseconds rather than seconds. Returns a restorer.
func noBackoff(t *testing.T) {
	t.Helper()
	prev := etxtbsyBackoff
	etxtbsyBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { etxtbsyBackoff = prev })
}

func TestIsTextFileBusy(t *testing.T) {
	t.Parallel() // only reads the function, never mutates package state
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("permission denied"), false},
		{errors.New("fork/exec /foo/bar: text file busy"), true},
		{errors.New("text file busy"), true},
		{errors.New("Unrecognized remote plugin message"), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, isTextFileBusy(c.err), "for err=%v", c.err)
	}
}

// retryOnTextFileBusy is the unit under test. It is exercised here without
// spinning up real plugin processes by injecting a fake pluginStarter.
//
// Issue #3425: under concurrent `pulumi install`, the bridge sporadically hits
// "text file busy" from execve. The fix retries up to etxtbsyMaxAttempts.
//
// These tests overwrite the package-level etxtbsyBackoff variable, so they
// cannot run in parallel with each other.

func TestRetryOnTextFileBusy_SucceedsFirstTry(t *testing.T) { //nolint:paralleltest
	noBackoff(t)

	var calls int
	start := func() (*plugin.Client, plugin.ClientProtocol, error) {
		calls++
		return nil, nil, nil
	}

	_, _, err := retryOnTextFileBusy(context.Background(), "p/q", start)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetryOnTextFileBusy_RecoversAfterRetry(t *testing.T) { //nolint:paralleltest
	noBackoff(t)

	var calls int
	busy := errors.New("fork/exec /cache/terraform-provider-x: text file busy")
	start := func() (*plugin.Client, plugin.ClientProtocol, error) {
		calls++
		if calls < 3 {
			return nil, nil, busy
		}
		return nil, nil, nil
	}

	_, _, err := retryOnTextFileBusy(context.Background(), "p/q", start)
	require.NoError(t, err)
	assert.Equal(t, 3, calls, "should have retried twice before succeeding")
}

func TestRetryOnTextFileBusy_NonRetryableErrorReturnsImmediately(t *testing.T) { //nolint:paralleltest
	noBackoff(t)

	other := errors.New("permission denied")
	var calls int
	start := func() (*plugin.Client, plugin.ClientProtocol, error) {
		calls++
		return nil, nil, other
	}

	_, _, err := retryOnTextFileBusy(context.Background(), "p/q", start)
	require.ErrorIs(t, err, other)
	assert.Equal(t, 1, calls, "non-ETXTBSY errors must not retry")
}

func TestRetryOnTextFileBusy_GivesUpAfterMaxAttempts(t *testing.T) { //nolint:paralleltest
	noBackoff(t)

	busy := errors.New("text file busy")
	var calls int
	start := func() (*plugin.Client, plugin.ClientProtocol, error) {
		calls++
		return nil, nil, busy
	}

	_, _, err := retryOnTextFileBusy(context.Background(), "registry.opentofu.org/hashicorp/random", start)
	require.Error(t, err)
	assert.ErrorIs(t, err, busy)
	assert.Contains(t, err.Error(), "registry.opentofu.org/hashicorp/random")
	assert.Equal(t, etxtbsyMaxAttempts, calls)
}

func TestRetryOnTextFileBusy_BackoffIsCalledBeforeRetries(t *testing.T) { //nolint:paralleltest
	var (
		calls         int
		backoffCalls  []int
		busy          = errors.New("text file busy")
		prevBackoff   = etxtbsyBackoff
		recordBackoff = func(attempt int) time.Duration {
			backoffCalls = append(backoffCalls, attempt)
			return 0
		}
	)
	etxtbsyBackoff = recordBackoff
	t.Cleanup(func() { etxtbsyBackoff = prevBackoff })

	start := func() (*plugin.Client, plugin.ClientProtocol, error) {
		calls++
		if calls < 2 {
			return nil, nil, busy
		}
		return nil, nil, nil
	}

	_, _, err := retryOnTextFileBusy(context.Background(), "p/q", start)
	require.NoError(t, err)
	// First attempt has no backoff; second attempt is preceded by one backoff call with attempt=1.
	assert.Equal(t, []int{1}, backoffCalls)
}

func Integration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping integration test during -short")
	}
}

// The use of t.Setenv makes it necessary to disable t.Parallel() and skip the paralleltest linter rule.
func TestLoadProvider(t *testing.T) { //nolint:paralleltest
	if runtime.GOOS != "windows" {
		// Do not cache during the test. This does not seem to work on Windows correctly due to temp dir cleanup
		// issues, therefore when running on Windows beware that the test may over-optimistically pass against
		// a cached result from the previous run.
		t.Setenv(envPluginCache, t.TempDir())
	}

	t.Run("registry", func(t *testing.T) {
		Integration(t)
		ctx := context.Background()

		p, err := NamedProvider(ctx, "hashicorp/tls", "<4.0.5,>4.0.3")
		require.NoError(t, err)

		require.Equal(t, "4.0.4", p.Version())

		resp, err := p.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
		require.NoError(t, err)

		assert.Equal(t, &tfprotov6.Schema{Block: &tfprotov6.SchemaBlock{
			Description:     "Provider configuration",
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Attributes:      []*tfprotov6.SchemaAttribute{},
			BlockTypes: []*tfprotov6.SchemaNestedBlock{{
				TypeName: "proxy",
				Block: &tfprotov6.SchemaBlock{
					Description:     "Proxy used by resources and data sources that connect to external endpoints.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Attributes: []*tfprotov6.SchemaAttribute{
						{
							Name: "from_env",
							Type: tftypes.Bool,
							//nolint:lll
							Description:     "When `true` the provider will discover the proxy configuration from environment variables. This is based upon [`http.ProxyFromEnvironment`](https://pkg.go.dev/net/http#ProxyFromEnvironment) and it supports the same environment variables (default: `true`).",
							Optional:        true,
							Computed:        true,
							DescriptionKind: tfprotov6.StringKindMarkdown,
						},
						{
							Name:            "password",
							Type:            tftypes.String,
							Description:     "Password used for Basic authentication against the Proxy.",
							DescriptionKind: tfprotov6.StringKindMarkdown,
							Optional:        true,
							Sensitive:       true,
						},
						{
							Name:            "url",
							Type:            tftypes.String,
							Description:     "URL used to connect to the Proxy. Accepted schemes are: `http`, `https`, `socks5`. ",
							DescriptionKind: tfprotov6.StringKindMarkdown,
							Optional:        true,
						},
						{
							Name:            "username",
							Type:            tftypes.String,
							Description:     "Username (or Token) used for Basic authentication against the Proxy.",
							DescriptionKind: tfprotov6.StringKindMarkdown,
							Optional:        true,
						},
					},
					BlockTypes: []*tfprotov6.SchemaNestedBlock{},
				},
				Nesting:  tfprotov6.SchemaNestedBlockNestingModeList,
				MaxItems: 1,
			}},
		}}, resp.Provider)
	})
}
