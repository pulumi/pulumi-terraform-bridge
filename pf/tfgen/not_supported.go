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

	u := &notSupportedUtil{sink}

	if prov.P != nil {
		u.warn("ProviderInfo.P should be nil for Plugin Framework based providers, populate NewProvider instead")
	}

	if prov.Resources != nil {
		for path, res := range prov.Resources {
			u.resource("resource:"+path, res)
		}
	}

	if prov.DataSources != nil {
		for path, ds := range prov.DataSources {
			u.datasource("datasource:"+path, ds)
		}
	}

	if prov.Config != nil {
		for path, ds := range prov.Config {
			u.schema("config:"+path, ds)
		}
	}

	u.assertNotZero("ExtraConfig", prov.ExtraConfig)
	u.assertNotZero("ExtraTypes", prov.ExtraTypes)
	u.assertNotZero("ExtraResources", prov.ExtraResources)
	u.assertNotZero("ExtraFunctions", prov.ExtraFunctions)
	u.assertNotZero("PreConfigureCallback", prov.PreConfigureCallback)
	u.assertNotZero("PreConfigureCallbackWithLogger", prov.PreConfigureCallbackWithLogger)

	return nil
}

type notSupportedUtil struct {
	sink diag.Sink
}

func (u *notSupportedUtil) warn(format string, arg ...interface{}) {
	u.sink.Warningf(&diag.Diag{Message: format}, arg...)
}

func (u *notSupportedUtil) assertNotZero(path string, shouldBeZero interface{}) {
	va := reflect.ValueOf(shouldBeZero)
	if va.IsZero() || va.IsNil() {
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
	u.assertNotZero(path+".UniqueNameFields", res.UniqueNameFields)
	u.assertNotZero(path+".Docs", res.Docs)
	u.assertNotZero(path+".DeleteBeforeReplace", res.DeleteBeforeReplace)
	u.assertNotZero(path+".Aliases", res.Aliases)
}

func (u *notSupportedUtil) schema(path string, schema *tfbridge.SchemaInfo) {
	u.assertNotZero(path+".Type", schema.Type)
	u.assertNotZero(path+".AltTypes", schema.AltTypes)
	u.assertNotZero(path+".NestedType", schema.NestedType)
	u.assertNotZero(path+".Transform", schema.Transform)
	u.assertNotZero(path+".Elem", schema.Elem)
	u.fields(path, schema.Fields)
	u.assertNotZero(path+".Asset", schema.Asset)
	u.assertNotZero(path+".Default", schema.Default)
	u.assertNotZero(path+".Stable", schema.Stable)
	u.assertNotZero(path+".SuppressEmptyMapElements", schema.SuppressEmptyMapElements)
	u.assertNotZero(path+".MarkAsComputedOnly", schema.MarkAsComputedOnly)
	u.assertNotZero(path+".MarkAsOptional", schema.MarkAsOptional)
	u.assertNotZero(path+".ForceNew", schema.ForceNew)
	u.assertNotZero(path+".Removed", schema.Removed)
	u.assertNotZero(path+".Secret", schema.Secret)
}
