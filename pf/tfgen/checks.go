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

package tfgen

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func checkProvider(sink diag.Sink, info tfbridge.ProviderInfo) error {
	isPFResource := func(string) bool { return true }
	isPFDataSource := func(string) bool { return true }
	if p, ok := info.P.(*muxer.ProviderShim); ok {
		isPFResource = p.ResourceIsPF
		isPFDataSource = p.DataSourceIsPF
	}

	return errors.Join(
		checkIDProperties(sink, info, isPFResource),
		notSupported(sink, info, isPFResource, isPFDataSource),
	)
}

func checkIDProperties(sink diag.Sink, info tfbridge.ProviderInfo, doCheck func(tfToken string) bool) error {
	errors := 0

	info.P.ResourcesMap().Range(func(rname string, resource shim.Resource) bool {
		if !doCheck(rname) {
			return true
		}
		if resourceHasComputeID(info, rname) {
			return true
		}
		ok, reason := resourceHasRegularID(resource)
		if ok {
			return true
		}
		m := fmt.Sprintf("Resource %s has a problem: %s. "+
			"To map this resource consider specifying ResourceInfo.ComputeID",
			rname, reason)
		errors++
		sink.Errorf(&diag.Diag{Message: m})

		return true
	})

	if errors > 0 {
		return fmt.Errorf("There were %d unresolved ID mapping errors", errors)
	}

	return nil
}

func resourceHasRegularID(resource shim.Resource) (bool, string) {
	idSchema, gotID := resource.Schema().GetOk("id")
	if !gotID {
		return false, `no "id" attribute`
	}
	if idSchema.Type() != shim.TypeString {
		return false, `"id" attribute is not of type String`
	}
	if idSchema.Sensitive() {
		return false, `"id" attribute is sensitive`
	}
	return true, ""
}

func resourceHasComputeID(info tfbridge.ProviderInfo, resname string) bool {
	if info.Resources == nil {
		return false
	}
	if info, ok := info.Resources[resname]; ok {
		return info.ComputeID != nil
	}
	return false
}
