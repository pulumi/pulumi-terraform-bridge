package crosstests

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FailNotEqual(t T, name string, tfVal, pulVal any) {
	t.Logf(name + " not equal!")
	t.Logf("TF value %s", tfVal)
	t.Logf("PU value %s", pulVal)
	t.Fail()
}

func assertCtyValEqual(t T, name string, tfVal, pulVal cty.Value) {
	if !tfVal.RawEquals(pulVal) {
		FailNotEqual(t, name, tfVal.GoString(), pulVal.GoString())
	}
}

func assertValEqual(t T, name string, tfVal, pulVal any) {
	// usually plugin-sdk schema types
	if hasEqualTfVal, ok := tfVal.(interface{ Equal(interface{}) bool }); ok {
		if !hasEqualTfVal.Equal(pulVal) {
			FailNotEqual(t, name, tfVal, pulVal)
		}
	} else {
		require.Equal(t, tfVal, pulVal, "Values for key %s do not match", name)
	}
}

func assertResourceDataEqual(t T, resourceSchema map[string]*schema.Schema, tfResult, puResult *schema.ResourceData) {
	// We are unable to assert that both providers were configured with the exact same
	// data. Type information doesn't line up in the simple case. This just doesn't work:
	//
	//	assert.Equal(t, tfResult, puResult)
	//
	// We make do by comparing raw data.

	t.Logf("tfResult: %+v", tfResult)
	t.Logf("puResult: %+v", puResult)

	assertCtyValEqual(t, "RawConfig", tfResult.GetRawConfig(), puResult.GetRawConfig())
	assertCtyValEqual(t, "RawPlan", tfResult.GetRawPlan(), puResult.GetRawPlan())
	assertCtyValEqual(t, "RawState", tfResult.GetRawState(), puResult.GetRawState())

	for _, timeout := range []string{
		schema.TimeoutCreate,
		schema.TimeoutRead,
		schema.TimeoutUpdate,
		schema.TimeoutDelete,
		schema.TimeoutDefault,
	} {
		assert.Equal(t, tfResult.Timeout(timeout), puResult.Timeout(timeout), "timeout %s", timeout)
	}

	for k := range resourceSchema {
		// TODO: make this recursive
		tfVal := tfResult.Get(k)
		pulVal := puResult.Get(k)

		tfChangeValOld, tfChangeValNew := tfResult.GetChange(k)
		pulChangeValOld, pulChangeValNew := puResult.GetChange(k)

		assertValEqual(t, k, tfVal, pulVal)
		assertValEqual(t, k+" Change Old", tfChangeValOld, pulChangeValOld)
		assertValEqual(t, k+" Change New", tfChangeValNew, pulChangeValNew)
	}
}
