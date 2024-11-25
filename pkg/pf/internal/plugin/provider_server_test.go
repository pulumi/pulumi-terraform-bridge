package plugin

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
)

func TestMarshalDiff(t *testing.T) {
	t.Parallel()

	runTest := func(t *testing.T, diff plugin.DiffResult) *pulumirpc.DiffResponse {
		server := providerServer{}
		resp, err := server.marshalDiff(diff)
		require.NoError(t, err)
		return resp
	}

	t.Run("no diffs", func(t *testing.T) {
		diff := plugin.DiffResult{
			Changes:      plugin.DiffNone,
			ReplaceKeys:  []resource.PropertyKey{},
			ChangedKeys:  []resource.PropertyKey{},
			DetailedDiff: map[string]plugin.PropertyDiff{},
		}

		require.Equal(t, &pulumirpc.DiffResponse{
			Replaces: []string{},
			Changes:  pulumirpc.DiffResponse_DIFF_NONE,
			Diffs:    []string{},
		}, runTest(t, diff))
	})

	t.Run("diff without detailed diff", func(t *testing.T) {
		diff := plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"replace"},
			ChangedKeys: []resource.PropertyKey{"change"},
		}

		require.Equal(t, &pulumirpc.DiffResponse{
			Replaces: []string{"replace"},
			Changes:  pulumirpc.DiffResponse_DIFF_SOME,
			Diffs:    []string{"change"},
		}, runTest(t, diff))
	})

	t.Run("diff with detailed diff", func(t *testing.T) {
		diff := plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"replace"},
			ChangedKeys: []resource.PropertyKey{"change", "replace"},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"change": {
					Kind: plugin.DiffAdd,
				},
				"replace": {
					Kind: plugin.DiffDeleteReplace,
				},
			},
		}

		require.Equal(t, &pulumirpc.DiffResponse{
			Replaces: []string{
				"replace",
			},
			Changes: pulumirpc.DiffResponse_DIFF_SOME,
			Diffs: []string{
				"change",
				"replace",
			},
			DetailedDiff: map[string]*pulumirpc.PropertyDiff{
				"change":  {},
				"replace": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
			},
			HasDetailedDiff: true,
		}, runTest(t, diff))
	})
}
