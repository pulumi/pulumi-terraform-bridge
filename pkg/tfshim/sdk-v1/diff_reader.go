package sdkv1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/mitchellh/mapstructure"
)

// diffFieldReader reads fields out of a diff structures.
//
// It also requires access to a Reader that reads fields from the structure
// that the diff was derived from. This is usually the state. This is required
// because a diff on its own doesn't have complete data about full objects
// such as maps.
//
// The Source MUST be the data that the diff was derived from. If it isn't,
// the behavior of this struct is undefined.
//
// Reading fields from a diffFieldReader is identical to reading from
// Source except the diff will be applied to the end result.
//
// The "Exists" field on the result will be set to true if the complete
// field exists whether its from the source, diff, or a combination of both.
// It cannot be determined whether a retrieved value is composed of
// diff elements.
type diffFieldReader struct {
	Diff   *terraform.InstanceDiff
	Source schema.FieldReader
	Schema map[string]*schema.Schema
}

func (r *diffFieldReader) ReadField(address []string) (schema.FieldReadResult, error) {
	schemaList := schema.SchemasForFlatmapPath(strings.Join(address, "."), r.Schema)
	if len(schemaList) == 0 {
		return schema.FieldReadResult{}, nil
	}

	var res schema.FieldReadResult
	var err error

	sch := schemaList[len(schemaList)-1]
	switch sch.Type {
	case schema.TypeBool, schema.TypeInt, schema.TypeFloat, schema.TypeString:
		res, err = r.readPrimitive(address, sch)
	case schema.TypeList:
		res, err = r.readListField(address, sch)
	case schema.TypeMap:
		res, err = r.readMap(address, sch)
	case schema.TypeSet:
		res, err = r.readSet(address, sch)
	default:
		res, err = r.readObjectField(address, sch.Elem.(map[string]*schema.Schema))
	}

	return res, err
}

// readListField is a generic method for reading a list field out of a
// a FieldReader. It does this based on the assumption that there is a key
// "foo.#" for a list "foo" and that the indexes are "foo.0", "foo.1", etc.
// after that point.
func (r *diffFieldReader) readListField(
	addr []string, sch *schema.Schema) (schema.FieldReadResult, error) {

	addrPadded := make([]string, len(addr)+1)
	copy(addrPadded, addr)
	addrPadded[len(addrPadded)-1] = "#"

	// Get the number of elements in the list
	countResult, err := r.ReadField(addrPadded)
	if err != nil {
		return schema.FieldReadResult{}, err
	}
	if !countResult.Exists {
		// No count, means we have no list
		countResult.Value = 0
	}

	// If we have an empty list, then return an empty list
	if countResult.Computed || countResult.Value.(int) == 0 {
		return schema.FieldReadResult{
			Value:    []interface{}{},
			Exists:   countResult.Exists,
			Computed: countResult.Computed,
		}, nil
	}

	// Go through each count, and get the item value out of it
	result := make([]interface{}, countResult.Value.(int))
	for i := range result {
		is := strconv.FormatInt(int64(i), 10)
		addrPadded[len(addrPadded)-1] = is
		rawResult, err := r.ReadField(addrPadded)
		if err != nil {
			return schema.FieldReadResult{}, err
		}
		if !rawResult.Exists {
			// This should never happen, because by the time the data
			// gets to the FieldReaders, all the defaults should be set by
			// Schema.
			rawResult.Value = nil
		}
		if rawResult.Computed {
			result[i] = UnknownVariableValue
		} else {
			result[i] = rawResult.Value
		}
	}

	return schema.FieldReadResult{
		Value:  result,
		Exists: true,
	}, nil
}

// readObjectField is a generic method for reading objects out of FieldReaders
// based on the assumption that building an address of []string{k, FIELD}
// will result in the proper field data.
func (r *diffFieldReader) readObjectField(
	addr []string, sch map[string]*schema.Schema) (schema.FieldReadResult, error) {

	result := make(map[string]interface{})
	exists := false
	for field, s := range sch {
		addrRead := make([]string, len(addr), len(addr)+1)
		copy(addrRead, addr)
		addrRead = append(addrRead, field)
		rawResult, err := r.ReadField(addrRead)
		if err != nil {
			return schema.FieldReadResult{}, err
		}
		if rawResult.Exists {
			exists = true
		}
		if rawResult.Computed {
			result[field] = UnknownVariableValue
		} else {
			result[field] = rawResult.ValueOrZero(s)
		}
	}

	return schema.FieldReadResult{
		Value:  result,
		Exists: exists,
	}, nil
}

func (r *diffFieldReader) readMap(
	address []string, sch *schema.Schema) (schema.FieldReadResult, error) {

	result := make(map[string]interface{})
	resultSet := false

	// First read the map from the underlying source
	source, err := r.Source.ReadField(address)
	if err != nil {
		return schema.FieldReadResult{}, err
	}
	if source.Exists {
		// readMap may return a nil value, or an unknown value placeholder in
		// some cases, causing the type assertion to panic if we don't assign the ok value
		result, _ = source.Value.(map[string]interface{})
		resultSet = true
	}

	// Next, read all the elements we have in our diff, and apply
	// the diff to our result.
	prefix := strings.Join(address, ".") + "."
	for k, v := range r.Diff.Attributes {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		if strings.HasPrefix(k, prefix+"%") {
			if v.NewComputed {
				return schema.FieldReadResult{
					Value:    result,
					Exists:   true,
					Computed: true,
				}, nil
			}

			// Ignore the count field unless it is computed.
			continue
		}

		resultSet = true

		k = k[len(prefix):]
		if v.NewRemoved {
			delete(result, k)
			continue
		}
		if v.NewComputed {
			result[k] = UnknownVariableValue
		} else {
			result[k] = v.New
		}
	}

	key := address[len(address)-1]
	err = r.mapValuesToPrimitive(key, result, sch)
	if err != nil {
		return schema.FieldReadResult{}, nil
	}

	var resultVal interface{}
	if resultSet {
		resultVal = result
	}

	return schema.FieldReadResult{
		Value:  resultVal,
		Exists: resultSet,
	}, nil
}

func (r *diffFieldReader) readPrimitive(
	address []string, sch *schema.Schema) (schema.FieldReadResult, error) {

	result, err := r.Source.ReadField(address)
	if err != nil {
		return schema.FieldReadResult{}, err
	}

	attrD, ok := r.Diff.Attributes[strings.Join(address, ".")]
	if !ok {
		return result, nil
	}

	var resultVal string
	if !attrD.NewComputed {
		resultVal = attrD.New
		if attrD.NewExtra != nil {
			result.ValueProcessed = resultVal
			if err := mapstructure.WeakDecode(attrD.NewExtra, &resultVal); err != nil {
				return schema.FieldReadResult{}, err
			}
		}
	}

	result.Computed = attrD.NewComputed
	result.Exists = true
	result.Value, err = r.stringToPrimitive(resultVal, sch)
	if err != nil {
		return schema.FieldReadResult{}, err
	}

	return result, nil
}

func (r *diffFieldReader) readSet(
	address []string, sch *schema.Schema) (schema.FieldReadResult, error) {

	// copy address to ensure we don't modify the argument
	address = append([]string(nil), address...)

	prefix := strings.Join(address, ".") + "."

	// Create the set that will be our result
	set := sch.ZeroValue().(*schema.Set)

	// Go through the map and find all the set items
	for k, d := range r.Diff.Attributes {
		if d.NewRemoved {
			// If the field is removed, we always ignore it
			continue
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		if strings.HasSuffix(k, "#") {
			// Ignore any count field
			continue
		}

		// Split the key, since it might be a sub-object like "idx.field"
		parts := strings.Split(k[len(prefix):], ".")
		idx := parts[0]

		// If the index begins with a '~', the value mst be computed.
		if !strings.HasPrefix(idx, "~") {
			raw, err := r.ReadField(append(address, idx))
			if err != nil {
				return schema.FieldReadResult{}, err
			}
			if !raw.Exists {
				// This shouldn't happen because we just verified it does exist
				panic("missing field in set: " + k + "." + idx)
			}

			if !raw.Computed {
				set.Add(raw.Value)
				continue
			}
		}

		// If any element of the set is computed, we must treat the whole set as computed.
		return schema.FieldReadResult{
			Value:    set,
			Exists:   true,
			Computed: true,
		}, nil
	}

	// Determine if the set "exists". It exists if there are items or if
	// the diff explicitly wanted it empty.
	exists := set.Len() > 0
	if !exists {
		// We could check if the diff value is "0" here but I think the
		// existence of "#" on its own is enough to show it existed. This
		// protects us in the future from the zero value changing from
		// "0" to "" breaking us (if that were to happen).
		if d, ok := r.Diff.Attributes[prefix+"#"]; ok {
			if d.NewComputed {
				return schema.FieldReadResult{
					Value:    set,
					Exists:   true,
					Computed: true,
				}, nil
			}

			exists = true
		}
	}

	if !exists {
		result, err := r.Source.ReadField(address)
		if err != nil {
			return schema.FieldReadResult{}, err
		}
		if result.Exists {
			return result, nil
		}
	}

	return schema.FieldReadResult{
		Value:  set,
		Exists: exists,
	}, nil
}

func (r *diffFieldReader) getValueType(k string, sch *schema.Schema) (schema.ValueType, error) {
	if sch.Elem == nil {
		return schema.TypeString, nil
	}
	if vt, ok := sch.Elem.(schema.ValueType); ok {
		return vt, nil
	}

	// If a Schema is provided to a Map, we use the Type of that schema
	// as the type for each element in the Map.
	if s, ok := sch.Elem.(*schema.Schema); ok {
		return s.Type, nil
	}

	if _, ok := sch.Elem.(*schema.Resource); ok {
		// TODO: We don't actually support this (yet)
		// but silently pass the validation, until we decide
		// how to handle nested structures in maps
		return schema.TypeString, nil
	}
	return 0, fmt.Errorf("%s: unexpected map value type: %#v", k, sch.Elem)
}

// convert map values to the proper primitive type based on schema.Elem
func (r *diffFieldReader) mapValuesToPrimitive(k string, m map[string]interface{}, sch *schema.Schema) error {
	elemType, err := r.getValueType(k, sch)
	if err != nil {
		return err
	}

	switch elemType {
	case schema.TypeInt, schema.TypeFloat, schema.TypeBool:
		for k, v := range m {
			vs, ok := v.(string)
			if !ok || v == UnknownVariableValue {
				continue
			}

			v, err := r.stringToPrimitive(vs, &schema.Schema{Type: elemType})
			if err != nil {
				return err
			}

			m[k] = v
		}
	}
	return nil
}

func (r *diffFieldReader) stringToPrimitive(
	value string, sch *schema.Schema) (interface{}, error) {
	var returnVal interface{}
	switch sch.Type {
	case schema.TypeBool:
		if value == "" {
			returnVal = false
			break
		}

		v, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}

		returnVal = v
	case schema.TypeFloat:
		if value == "" {
			returnVal = 0.0
			break
		}

		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}

		returnVal = v
	case schema.TypeInt:
		if value == "" {
			returnVal = 0
			break
		}

		v, err := strconv.ParseInt(value, 0, 0)
		if err != nil {
			return nil, err
		}

		returnVal = int(v)
	case schema.TypeString:
		returnVal = value
	default:
		panic(fmt.Errorf("Unknown type: %v", sch.Type))
	}

	return returnVal, nil
}
