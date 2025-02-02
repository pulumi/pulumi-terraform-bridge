// Code copied from github.com/opentofu/opentofu by go generate; DO NOT EDIT.
// Code generated by "stringer -type=NestingMode"; DO NOT EDIT.

package configschema

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[nestingModeInvalid-0]
	_ = x[NestingSingle-1]
	_ = x[NestingGroup-2]
	_ = x[NestingList-3]
	_ = x[NestingSet-4]
	_ = x[NestingMap-5]
}

const _NestingMode_name = "nestingModeInvalidNestingSingleNestingGroupNestingListNestingSetNestingMap"

var _NestingMode_index = [...]uint8{0, 18, 31, 43, 54, 64, 74}

func (i NestingMode) String() string {
	if i < 0 || i >= NestingMode(len(_NestingMode_index)-1) {
		return "NestingMode(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _NestingMode_name[_NestingMode_index[i]:_NestingMode_index[i+1]]
}
