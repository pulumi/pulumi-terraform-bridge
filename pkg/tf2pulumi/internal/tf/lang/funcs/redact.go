package funcs

import (
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/lang/marks"
	"github.com/zclconf/go-cty/cty"
)

func redactIfSensitive(value interface{}, markses ...cty.ValueMarks) string {
	if marks.Has(cty.DynamicVal.WithMarks(markses...), marks.Sensitive) {
		return "(sensitive value)"
	}
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
