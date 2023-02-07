// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package convert

import "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

func functionOutputs(spec *schema.FunctionSpec) *schema.ObjectTypeSpec {
	switch {
	case spec.Outputs != nil && spec.ReturnType == nil:
		return spec.Outputs
	case spec.ReturnType != nil && spec.Outputs == nil:
		r := spec.ReturnType
		switch {
		case r.ObjectTypeSpec != nil && r.TypeSpec == nil:
			return spec.ReturnType.ObjectTypeSpec
		case r.TypeSpec != nil && r.ObjectTypeSpec == nil:
			panic("TODO[pulumi/pulumi-terraform-bridge#796] TypeSpec is not supported yet by pf/tfbridge")
		default:
			panic("FunctionSpec.ReturnType should have either ObjectTypeSpec or TypeSpec")
		}
	default:
		panic("FunctionSpec should have either ReturnType or Outputs")
	}
}
