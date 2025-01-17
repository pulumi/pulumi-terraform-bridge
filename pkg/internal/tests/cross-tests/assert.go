package crosstests

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"
)

func assertValEqual(t T, name string, tfVal, pulVal any) {
	// usually plugin-sdk schema types
	if hasEqualTfVal, ok := tfVal.(interface{ Equal(interface{}) bool }); ok {
		if !hasEqualTfVal.Equal(pulVal) {
			t.Logf(name + " not equal!")
			t.Logf("TF value %s", tfVal)
			t.Logf("PU value %s", pulVal)
			t.Fail()
		}
	} else {
		require.Equal(t, tfVal, pulVal, "Values for key %s do not match", name)
	}
}

func assertResourceDataEqual(t T, tfResult, puResult *schema.ResourceData) {
	// Use cmp to check if data is equal. We need to use cmp instead of
	// `assert`'s default `reflect.DeepEqual` because cmp treats identical
	// function pointers as equal, but `reflect.DeepEqual` does not.
	opts := []cmp.Option{
		cmp.Exporter(func(reflect.Type) bool { return true }),
		cmp.Comparer(func(x, y schema.SchemaStateFunc) bool {
			return reflect.ValueOf(x).Pointer() == reflect.ValueOf(y).Pointer()
		}),
	}
	if !cmp.Equal(tfResult, puResult, opts...) {
		t.Logf("Diff: %s", cmp.Diff(tfResult, puResult, opts...))
		t.Fail()
	}
}
