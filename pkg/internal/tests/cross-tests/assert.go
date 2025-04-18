package crosstests

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
