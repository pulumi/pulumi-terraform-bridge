package crosstestsimpl

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/providertest/grpclog"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
)

type PulumiDiffResp struct {
	DetailedDiff        map[string]interface{} `json:"detailedDiff"`
	DeleteBeforeReplace bool                   `json:"deleteBeforeReplace"`
}

type DiffResult struct {
	TFDiff     tfcheck.TFChange
	PulumiDiff PulumiDiffResp
	// TFOut is the stdout of the terraform plan command
	TFOut string
	// PulumiOut is the stdout of the pulumi preview command
	PulumiOut string
}

func VerifyBasicDiffAgreement(t T, tfActions []string, us auto.UpdateSummary, diffResponse PulumiDiffResp) {
	t.Helper()
	t.Logf("UpdateSummary.ResourceChanges: %#v", us.ResourceChanges)
	// Action list from https://github.com/opentofu/opentofu/blob/main/internal/plans/action.go#L11
	if len(tfActions) == 0 {
		require.FailNow(t, "No TF actions found")
	}
	if len(tfActions) == 1 {
		switch tfActions[0] {
		case "no-op":
			require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
			rc := *us.ResourceChanges
			assert.Equalf(t, 2, rc[string(apitype.OpSame)], "expected the test resource and stack to stay the same")
			assert.Equalf(t, 1, len(rc), "expected one entry in UpdateSummary.ResourceChanges")
		case "create":
			require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
			rc := *us.ResourceChanges
			assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected the stack to stay the same")
			assert.Equalf(t, 1, rc[string(apitype.OpCreate)], "expected the test resource to get a create plan")
		case "read":
			require.FailNow(t, "Unexpected TF action: read")
		case "update":
			require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
			rc := *us.ResourceChanges
			assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected one resource to stay the same - the stack")
			assert.Equalf(t, 1, rc[string(apitype.Update)], "expected the test resource to get an update plan")
			assert.Equalf(t, 2, len(rc), "expected two entries in UpdateSummary.ResourceChanges")
		case "delete":
			require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
			rc := *us.ResourceChanges
			assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected the stack to stay the same")
			assert.Equalf(t, 1, rc[string(apitype.OpDelete)], "expected the test resource to get a delete plan")
		default:
			panic("TODO: do not understand this TF action yet: " + tfActions[0])
		}
	} else if len(tfActions) == 2 {
		if tfActions[0] == "create" && tfActions[1] == "delete" {
			require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
			rc := *us.ResourceChanges
			assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected the stack to stay the same")
			assert.Equalf(t, 1, rc[string(apitype.OpReplace)], "expected the test resource to get a replace plan")
			assert.Equalf(t, false, diffResponse.DeleteBeforeReplace, "expected deleteBeforeReplace to be true")
		} else if tfActions[0] == "delete" && tfActions[1] == "create" {
			require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
			rc := *us.ResourceChanges
			t.Logf("UpdateSummary.ResourceChanges: %#v", rc)
			assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected the stack to stay the same")
			assert.Equalf(t, 1, rc[string(apitype.OpReplace)], "expected the test resource to get a replace plan")
			assert.Equalf(t, true, diffResponse.DeleteBeforeReplace, "expected deleteBeforeReplace to be true")
		} else {
			panic("TODO: do not understand this TF action yet: " + fmt.Sprint(tfActions))
		}
	} else {
		panic("TODO: do not understand this TF action yet: " + fmt.Sprint(tfActions))
	}
}

func GetPulumiDiffResponse(t T, entries []grpclog.GrpcLogEntry) PulumiDiffResp {
	diffResponse := PulumiDiffResp{}
	found := false
	for _, entry := range entries {
		if entry.Method == "/pulumirpc.ResourceProvider/Diff" {
			if entry.Response == nil {
				continue
			}
			require.False(t, found, "expected to find only one Diff entry in the gRPC log")
			err := json.Unmarshal(entry.Response, &diffResponse)
			require.NoError(t, err)
			found = true
		}
	}

	return diffResponse
}
