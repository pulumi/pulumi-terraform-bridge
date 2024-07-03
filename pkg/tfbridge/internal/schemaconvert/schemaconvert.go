package schemaconvert

import (
	v1Schema "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	v2Schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func Sdkv2ToV1Type(t v2Schema.ValueType) v1Schema.ValueType {
	return v1Schema.ValueType(t)
}

func Sdkv2ToV1SchemaOrResource(elem interface{}) interface{} {
	switch elem := elem.(type) {
	case nil:
		return nil
	case *v2Schema.Schema:
		return Sdkv2ToV1Schema(elem)
	case *v2Schema.Resource:
		return Sdkv2ToV1Resource(elem)
	default:
		contract.Failf("unexpected type %T", elem)
		return nil
	}
}

func Sdkv2ToV1Resource(sch *v2Schema.Resource) *v1Schema.Resource {
	if sch.MigrateState != nil {
		contract.Failf("MigrateState is not supported in conversion")
	}
	if sch.StateUpgraders != nil {
		contract.Failf("StateUpgraders is not supported in conversion")
	}
	if sch.Create != nil || sch.Read != nil || sch.Update != nil || sch.Delete != nil || sch.Exists != nil ||
		sch.CreateContext != nil || sch.ReadContext != nil || sch.UpdateContext != nil ||
		sch.DeleteContext != nil || sch.Importer != nil {
		contract.Failf("runtime methods not supported in conversion")
	}

	if sch.CustomizeDiff != nil {
		contract.Failf("CustomizeDiff is not supported in conversion")
	}

	timeouts := v1Schema.ResourceTimeout{}
	if sch.Timeouts != nil {
		timeouts = v1Schema.ResourceTimeout{
			Create:  sch.Timeouts.Create,
			Read:    sch.Timeouts.Read,
			Update:  sch.Timeouts.Update,
			Delete:  sch.Timeouts.Delete,
			Default: sch.Timeouts.Default,
		}
	}
	timoutsPtr := &timeouts
	if sch.Timeouts == nil {
		timoutsPtr = nil
	}

	return &v1Schema.Resource{
		Schema:             Sdkv2ToV1SchemaMap(sch.Schema),
		SchemaVersion:      sch.SchemaVersion,
		DeprecationMessage: sch.DeprecationMessage,
		Timeouts:           timoutsPtr,
	}
}

func Sdkv2ToV1Schema(sch *v2Schema.Schema) *v1Schema.Schema {
	if sch.DiffSuppressFunc != nil {
		contract.Failf("DiffSuppressFunc is not supported in conversion")
	}

	defaultFunc := v1Schema.SchemaDefaultFunc(nil)
	if sch.DefaultFunc != nil {
		defaultFunc = func() (interface{}, error) {
			return sch.DefaultFunc()
		}
	}

	stateFunc := v1Schema.SchemaStateFunc(nil)
	if sch.StateFunc != nil {
		stateFunc = func(i interface{}) string {
			return sch.StateFunc(i)
		}
	}

	set := v1Schema.SchemaSetFunc(nil)
	if sch.Set != nil {
		set = func(i interface{}) int {
			return sch.Set(i)
		}
	}

	validateFunc := v1Schema.SchemaValidateFunc(nil)
	if sch.ValidateFunc != nil {
		validateFunc = func(i interface{}, s string) ([]string, []error) {
			return sch.ValidateFunc(i, s)
		}
	}

	return &v1Schema.Schema{
		Type:          Sdkv2ToV1Type(sch.Type),
		Optional:      sch.Optional,
		Required:      sch.Required,
		Default:       sch.Default,
		DefaultFunc:   defaultFunc,
		Description:   sch.Description,
		InputDefault:  sch.InputDefault,
		Computed:      sch.Computed,
		ForceNew:      sch.ForceNew,
		StateFunc:     stateFunc,
		Elem:          Sdkv2ToV1SchemaOrResource(sch.Elem),
		MaxItems:      sch.MaxItems,
		MinItems:      sch.MinItems,
		Set:           set,
		ComputedWhen:  sch.ComputedWhen,
		ConflictsWith: sch.ConflictsWith,
		ExactlyOneOf:  sch.ExactlyOneOf,
		AtLeastOneOf:  sch.AtLeastOneOf,
		Deprecated:    sch.Deprecated,
		ValidateFunc:  validateFunc,
		Sensitive:     sch.Sensitive,
	}
}

func Sdkv2ToV1SchemaMap(sch map[string]*v2Schema.Schema) map[string]*v1Schema.Schema {
	res := make(map[string]*v1Schema.Schema)
	for k, v := range sch {
		res[k] = Sdkv2ToV1Schema(v)
	}
	return res
}
