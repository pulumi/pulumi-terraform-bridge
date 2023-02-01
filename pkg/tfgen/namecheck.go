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
	"fmt"
	"os"
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type nameErrors []nameError

func (ne nameErrors) Sort() {
	sort.Slice(ne, func(i, j int) bool {
		if ne[i].context < ne[j].context {
			return true
		}
		if ne[i].tfKey < ne[j].tfKey {
			return true
		}
		if ne[i].tfAttr < ne[j].tfAttr {
			return true
		}
		if ne[i].message < ne[j].message {
			return true
		}
		return false
	})
}

func (ne nameErrors) Report(sink diag.Sink) {
	if len(ne) == 0 {
		return
	}

	if sink == nil {
		sink = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Always,
		})
	}

	ne.Sort()
	sink.Warningf(&diag.Diag{Message: "Found %d naming error(s)"}, len(ne))
	for _, n := range ne {
		sink.Warningf(&diag.Diag{Message: "Naming error in Terraform %s %q attribute %q: %s"},
			n.context, n.tfKey, n.tfAttr, n.message)
	}
}

type nameContext uint16

const (
	configPropertyNames     nameContext = 1
	dataSourcePropertyNames nameContext = 2
	resourcePropertyNames   nameContext = 3
)

func (nc nameContext) String() string {
	switch nc {
	case configPropertyNames:
		return "Config"
	case dataSourcePropertyNames:
		return "DataSource"
	case resourcePropertyNames:
		return "Resource"
	}
	return "unknown"
}

type nameError struct {
	context nameContext
	tfKey   string
	tfAttr  string
	message string
}

func newNameTurnaroundError(nc nameContext, tfKey, tfAttr, pulumiName, tfAttrRecovered string) nameError {
	m := fmt.Sprintf("Pulumi name %q incorrectly translates back to Terraform as %q",
		pulumiName, tfAttrRecovered)
	return nameError{context: nc, tfKey: tfKey, tfAttr: tfAttr, message: m}
}

func newNameCollisionError(nc nameContext, tfKey, tfAttr, pulumiName, tfAttrOther string) nameError {
	m := fmt.Sprintf("Pulumi name %q already represents another Terraform name %q",
		pulumiName, tfAttrOther)
	return nameError{context: nc, tfKey: tfKey, tfAttr: tfAttr, message: m}
}

func validatePropertyNamesForProvider(prov tfbridge.ProviderInfo) nameErrors {
	errors := []nameError{}

	prov.P.Schema().Range(func(tfConfigKey string, sch shim.Schema) bool {
		es := validatePropertyNames(configPropertyNames, tfConfigKey, prov.P.Schema(), prov.Config)
		errors = append(errors, es...)
		return true
	})

	prov.P.ResourcesMap().Range(func(tfResKey string, res shim.Resource) bool {
		info := prov.Resources[tfResKey]
		if info == nil {
			info = &tfbridge.ResourceInfo{}
		}
		res.Schema()
		es := validatePropertyNames(resourcePropertyNames, tfResKey, res.Schema(), info.Fields)
		errors = append(errors, es...)
		return true
	})

	prov.P.DataSourcesMap().Range(func(tfDsKey string, ds shim.Resource) bool {
		info := prov.DataSources[tfDsKey]
		if info == nil {
			info = &tfbridge.DataSourceInfo{}
		}
		es := validatePropertyNames(dataSourcePropertyNames, tfDsKey, ds.Schema(), info.Fields)
		errors = append(errors, es...)
		return true
	})

	return nameErrors(errors)
}

func validatePropertyNames(
	nc nameContext,
	tfKey string,
	schemaMap shim.SchemaMap,
	infos map[string]*tfbridge.SchemaInfo,
) []nameError {
	errors := []nameError{}
	mapping := map[string]string{}
	schemaMap.Range(func(tfAttr string, tfAttrSchema shim.Schema) bool {
		var info *tfbridge.SchemaInfo
		if infos != nil {
			info = infos[tfAttr]
		}
		pulumiName := tfbridge.TerraformToPulumiName(tfAttr, tfAttrSchema, info, false)

		if oldAttr, exists := mapping[pulumiName]; exists {
			errors = append(errors, newNameCollisionError(nc, tfKey, tfAttr, pulumiName, oldAttr))
		}

		mapping[pulumiName] = tfAttr

		tfAttrRecovered := tfbridge.PulumiToTerraformName(pulumiName, schemaMap, infos)
		if tfAttrRecovered != tfAttr {
			errors = append(errors, newNameTurnaroundError(nc, tfKey, tfAttr, pulumiName, tfAttrRecovered))
		}

		return true

	})
	return errors
}
