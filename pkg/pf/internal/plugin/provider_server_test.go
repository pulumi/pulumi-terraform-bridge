package plugin

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
)

// autogold gets confused about the enums, so we just re-define them here.
//
//nolint:revive
const (
	ADD            pulumirpc.PropertyDiff_Kind = 0
	ADD_REPLACE    pulumirpc.PropertyDiff_Kind = 1
	DELETE         pulumirpc.PropertyDiff_Kind = 2
	DELETE_REPLACE pulumirpc.PropertyDiff_Kind = 3
	UPDATE         pulumirpc.PropertyDiff_Kind = 4
	UPDATE_REPLACE pulumirpc.PropertyDiff_Kind = 5
)

//nolint:revive
const (
	DIFF_UNKNOWN pulumirpc.DiffResponse_DiffChanges = 0
	DIFF_NONE    pulumirpc.DiffResponse_DiffChanges = 1
	DIFF_SOME    pulumirpc.DiffResponse_DiffChanges = 2
)

//nolint:unconvert
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

		autogold.Expect(&pulumirpc.DiffResponse{
			Replaces: []string{},
			Changes:  pulumirpc.DiffResponse_DIFF_NONE,
			Diffs:    []string{},
		}).Equal(t, runTest(t, diff))
	})

	t.Run("diff without detailed diff", func(t *testing.T) {
		diff := plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"replace"},
			ChangedKeys: []resource.PropertyKey{"change"},
		}

		autogold.Expect(&pulumirpc.DiffResponse{
			Replaces: []string{"replace"},
			Changes:  pulumirpc.DiffResponse_DIFF_SOME,
			Diffs:    []string{"change"},
		}).Equal(t, runTest(t, diff))
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

		autogold.Expect(&pulumirpc.DiffResponse{
			Replaces: []string{
				"replace",
			},
			Changes: pulumirpc.DiffResponse_DiffChanges(DIFF_SOME),
			Diffs: []string{
				"change",
				"replace",
			},
			DetailedDiff: map[string]*pulumirpc.PropertyDiff{
				"change":  {},
				"replace": {Kind: pulumirpc.PropertyDiff_Kind(DELETE_REPLACE)},
			},
			HasDetailedDiff: true,
		}).Equal(t, runTest(t, diff))
	})
}
