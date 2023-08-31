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
	"os"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/muxer"
	schemaShim "github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// Check if the user has customiezed ProviderInfo asking for features that are not yet supported for Plugin Framework
// based providers, emit warnings in this case.
func notSupported(sink diag.Sink, prov tfbridge.ProviderInfo) error {
	if sink == nil {
		sink = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Always,
		})
	}

	u := &notSupportedUtil{sink: sink}

	skipResource := func(tfToken string) bool { return false }
	skipDataSource := func(tfToken string) bool { return false }
	muxedProvider := false
	if mixed, ok := prov.P.(*muxer.ProviderShim); ok {
		not := func(f func(string) bool) func(string) bool {
			return func(s string) bool {
				return !f(s)
			}
		}

		skipResource = not(mixed.ResourceIsPF)
		skipDataSource = not(mixed.DataSourceIsPF)
		muxedProvider = true
	} else if _, ok := prov.P.(*schemaShim.SchemaOnlyProvider); !ok {
		warning := "Bridged Plugin Framework providers must have ProviderInfo.P be created from" +
			" pf/tfbridge.ShimProvider or pf/tfbridge.ShimProviderWithContext.\nMuxed SDK and" +
			" Plugin Framework based providers must have ProviderInfo.P be created from" +
			" pf/tfbridge.MuxShimWithPF or pf/tfbridgepf/tfbridge.MuxShimWithDisjointgPF."
		u.warn(warning)
	}

	if prov.Resources != nil {
		for path, res := range prov.Resources {
			if skipResource(path) {
				continue
			}
			u.resource("resource:"+path, res)
		}
	}

	if prov.DataSources != nil {
		for path, ds := range prov.DataSources {
			if skipDataSource(path) {
				continue
			}
			u.datasource("datasource:"+path, ds)
		}
	}

	// It might be reasonable to set global values that PF will ignore if this is a
	// muxed provider, and the SDK side will pick it up.
	if !muxedProvider {
		if prov.Config != nil {
			for path, ds := range prov.Config {
				u.schema("config:"+path, ds)
			}
		}
	}

	return nil
}

type notSupportedUtil struct {
	sink diag.Sink
}

func (u *notSupportedUtil) warn(format string, arg ...interface{}) {
	u.sink.Warningf(&diag.Diag{Message: format}, arg...)
}

func (u *notSupportedUtil) assertIsZero(path string, shouldBeZero interface{}) {
	va := reflect.ValueOf(shouldBeZero)
	if va.IsZero() {
		return
	}
	u.warn("%s received a non-zero custom value %v that is being ignored."+
		" Plugin Framework based providers do not yet support this feature.",
		path, shouldBeZero)
}

func (u *notSupportedUtil) fields(path string, f map[string]*tfbridge.SchemaInfo) {
	for k, v := range f {
		u.schema(path+".Fields."+k, v)
	}
}

func (u *notSupportedUtil) datasource(path string, ds *tfbridge.DataSourceInfo) {
	u.fields(path, ds.Fields)
}

func (u *notSupportedUtil) resource(path string, res *tfbridge.ResourceInfo) {
	u.fields(path, res.Fields)
	u.assertIsZero(path+".UniqueNameFields", res.UniqueNameFields)
	u.assertIsZero(path+".Docs", res.Docs)
	u.assertIsZero(path+".Aliases", res.Aliases)
}

func (u *notSupportedUtil) schema(path string, schema *tfbridge.SchemaInfo) {
	u.assertIsZero(path+".Type", schema.Type)
	u.assertIsZero(path+".AltTypes", schema.AltTypes)
	u.assertIsZero(path+".NestedType", schema.NestedType)
	u.assertIsZero(path+".Transform", schema.Transform)
	u.assertIsZero(path+".Elem", schema.Elem)
	u.fields(path, schema.Fields)
	u.assertIsZero(path+".Asset", schema.Asset)
	u.assertIsZero(path+".Stable", schema.Stable)
	u.assertIsZero(path+".SuppressEmptyMapElements", schema.SuppressEmptyMapElements)
	u.assertIsZero(path+".ForceNew", schema.ForceNew)
	u.assertIsZero(path+".Removed", schema.Removed)
}
