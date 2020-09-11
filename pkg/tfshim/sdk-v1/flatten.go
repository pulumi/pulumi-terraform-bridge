package sdkv1

import (
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// flattenValue takes a single value and recursively flattens its properties into the given string -> string map under
// the provided prefix. It expects that the value has been "schema-fied" by being read out of a schema.FieldReader (in
// particular, all sets *must* be represented as schema.Set values). The flattened value may then be used as the value
// of a terraform.InstanceState.Attributes field.
//
// Note that this duplicates much of the logic in TF's schema.MapFieldWriter. Ideally, we would just use that type,
// but there are various API/implementation challenges that preclude that option. The most worrying (and potentially
// fragile) piece of duplication is the code that calculates a set member's hash code; see the code under
// `case *schema.Set`.
func flattenValue(result map[string]string, prefix string, value interface{}) {
	if value == nil {
		return
	}

	switch t := value.(type) {
	case bool:
		if t {
			result[prefix] = "true"
		} else {
			result[prefix] = "false"
		}
	case int:
		result[prefix] = strconv.FormatInt(int64(t), 10)
	case float64:
		result[prefix] = strconv.FormatFloat(t, 'G', -1, 64)
	case string:
		result[prefix] = t
	case []interface{}:
		// flatten each element.
		for i, elem := range t {
			flattenValue(result, prefix+"."+strconv.FormatInt(int64(i), 10), elem)
		}

		// Set the count.
		result[prefix+".#"] = strconv.FormatInt(int64(len(t)), 10)
	case *schema.Set:
		// flatten each element.
		setList := t.List()
		for _, elem := range setList {
			// Note that the logic below is duplicated from `scheme.Set.hash`. If that logic ever changes, this will
			// need to change in kind.
			code := t.F(elem)
			if code < 0 {
				code = -code
			}

			flattenValue(result, prefix+"."+strconv.Itoa(code), elem)
		}

		// Set the count.
		result[prefix+".#"] = strconv.FormatInt(int64(len(setList)), 10)
	case map[string]interface{}:
		for k, v := range t {
			flattenValue(result, prefix+"."+k, v)
		}

		// Set the count.
		result[prefix+".%"] = strconv.Itoa(len(t))
	default:
		contract.Failf("Unexpected TF input value: %v", t)
	}
}
