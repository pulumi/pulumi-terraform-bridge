// Copyright 2016-2024, Pulumi Corporation.
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

// This file lets the bridge target the upstream
// github.com/hashicorp/terraform-plugin-sdk/v2 module directly instead of the
// github.com/pulumi/terraform-plugin-sdk fork.
//
// The fork existed to (a) re-export a handful of functions that live in the
// SDK's internal/ packages and (b) expose the *terraform.InstanceDiff that
// PlanResourceChange computes. We recover (a) here with //go:linkname, which
// binds these locally-declared functions to the unexported (or internal-only)
// symbols at link time. (b) is handled in plan_resource_change.go by computing
// the diff via the public Resource.SimpleDiff API.
//
// These linknames are an intentional coupling to SDK internals. If an SDK
// upgrade renames or removes one of these symbols the build fails at link time;
// linkname_test.go exercises every one so that failure surfaces as a test
// failure rather than at runtime.
package sdkv2

import (
	"context"
	"unsafe"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	// Pull the SDK packages that define the linkname targets into the build
	// graph so the symbols below resolve.
	_ "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// HCL2ValueFromConfigValue converts a Go value decoded from a flatmap into a
// cty.Value without consulting a schema. It mirrors the fork's re-export of
// internal/configs/hcl2shim.HCL2ValueFromConfigValue.
//
//go:linkname hcl2ValueFromConfigValue github.com/hashicorp/terraform-plugin-sdk/v2/internal/configs/hcl2shim.HCL2ValueFromConfigValue
func hcl2ValueFromConfigValue(v interface{}) cty.Value

//go:linkname hcl2ValueFromFlatmap github.com/hashicorp/terraform-plugin-sdk/v2/internal/configs/hcl2shim.HCL2ValueFromFlatmap
func hcl2ValueFromFlatmap(m map[string]string, ty cty.Type) (cty.Value, error)

//go:linkname valuesSDKEquivalent github.com/hashicorp/terraform-plugin-sdk/v2/internal/configs/hcl2shim.ValuesSDKEquivalent
func valuesSDKEquivalent(a, b cty.Value) bool

//go:linkname normalizeNullValues github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema.normalizeNullValues
func normalizeNullValues(dst, src cty.Value, apply bool) cty.Value

//go:linkname copyTimeoutValues github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema.copyTimeoutValues
func copyTimeoutValues(to, from cty.Value) cty.Value

// setWriteOnlyNullValues takes a *configschema.Block as its second argument.
// That type lives in an internal package and cannot be named here, so the
// pointer is passed as unsafe.Pointer; a plain pointer and unsafe.Pointer share
// the same calling convention, so the linked function reinterprets it as the
// *configschema.Block it expects. Callers pass the result of
// (*schema.Resource).CoreConfigSchema() unchanged.
//
//go:linkname setWriteOnlyNullValues github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema.setWriteOnlyNullValues
func setWriteOnlyNullValues(val cty.Value, block unsafe.Pointer) cty.Value

//go:linkname validateConfigNulls github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema.validateConfigNulls
func validateConfigNulls(ctx context.Context, v cty.Value, path cty.Path) []*tfprotov5.Diagnostic

// newExtraKey is the private-state key under which the SDK stashes StateFunc
// modified config values. It is an unexported const in helper/schema; the value
// is part of the on-disk private state format and is therefore stable.
const newExtraKey = "_new_extra_shim"
