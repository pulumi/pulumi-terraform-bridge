// Code copied from github.com/opentofu/opentofu by go generate; DO NOT EDIT.
// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package configschema

import (
	

	
	"github.com/zclconf/go-cty/cty"
)

// copyAndExtendPath returns a copy of a cty.Path with some additional
// `cty.PathStep`s appended to its end, to simplify creating new child paths.
func copyAndExtendPath(path cty.Path, nextSteps ...cty.PathStep) cty.Path {
	newPath := make(cty.Path, len(path), len(path)+len(nextSteps))
	copy(newPath, path)
	newPath = append(newPath, nextSteps...)
	return newPath
}

