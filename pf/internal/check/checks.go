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

package check

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Validate that info is valid as either a PF provider or a PF & SDK based provider.
//
// This function should be called in the generate step, but before schema generation (so
// as to error as soon as possible).
func Provider(sink diag.Sink, info tfbridge.ProviderInfo) error {
	// If info.P is not muxed, we assume that all resources are PF based resources and
	// that all datasources are PF based datasources.
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

func checkIDProperties(sink diag.Sink, info tfbridge.ProviderInfo, isPFResource func(tfToken string) bool) error {
	errors := 0

	info.P.ResourcesMap().Range(func(rname string, resource shim.Resource) bool {
		// If a resource is sdk based, it always has an ID, regardless of the
		// schema it describes.
		if !isPFResource(rname) {
			return true
		}
		if resourceHasComputeID(info, rname) {
			return true
		}
		err := resourceHasRegularID(rname, resource, info.Resources[rname])
		if err == nil {
			return true
		}

		errors++
		sink.Errorf(&diag.Diag{Message: resourceError{rname, err}.Error()})

		return true
	})

	if errors > 0 {
		return fmt.Errorf("There were %d unresolved ID mapping errors", errors)
	}

	return nil
}

type resourceError struct {
	token string
	err   error
}

func (err resourceError) Error() string {
	msg := fmt.Sprintf("Resource %s has a problem", err.token)
	if err.err != nil {
		msg += ": " + err.err.Error()
	}
	return msg
}

type errSensitiveID struct {
	token string
}

func (err errSensitiveID) Error() string {
	msg := `"id" attribute is sensitive, but cannot be kept secret.`
	if err.token != "" {
		msg += fmt.Sprintf(
			" To accept exposing ID, set `ProviderInfo.Resources[%q].Fields[%q].Secret = tfbridge.True()`",
			err.token, "id")
	}
	return msg
}

type errWrongIDType struct {
	actualType string
}

func (err errWrongIDType) Error() string {
	msg := `"id" attribute is not of type "string"`
	const postfix = ". To map this resource consider overriding the SchemaInfo.Type" +
		" field or specifying ResourceInfo.ComputeID"
	if err.actualType != "" {
		msg = fmt.Sprintf(
			`"id" attribute is of type %q, expected type "string"`,
			err.actualType)
	}
	return msg + postfix
}

type errMissingIDAttribute struct{}

func (errMissingIDAttribute) Error() string {
	return `no "id" attribute. To map this resource consider specifying ResourceInfo.ComputeID`
}

type errInvalidRequiredID struct{}

func (errInvalidRequiredID) Error() string {
	return `a required "id" input attribute is not allowed.` +
		"To map this resource specify SchemaInfo.Name and ResourceInfo.ComputeID"
}

func isInputProperty(schema shim.Schema) bool {
	if schema.Computed() && !schema.Optional() {
		return false
	}
	return true
}

func resourceHasRegularID(rname string, resource shim.Resource, resourceInfo *tfbridge.ResourceInfo) error {
	idSchema, gotID := resource.Schema().GetOk("id")
	if !gotID {
		return errMissingIDAttribute{}
	}
	var info tfbridge.SchemaInfo
	if resourceInfo != nil {
		if id := resourceInfo.Fields["id"]; id != nil {
			info = *id
		}
	}

	if isInputProperty(idSchema) && (info.Name == "" || resourceInfo.ComputeID == nil) {
		return errInvalidRequiredID{}
	}

	// If the user over-rode the type to be a string, don't reject.
	if idSchema.Type() != shim.TypeString && info.Type != "string" {
		actual := idSchema.Type().String()
		if info.Type != "" {
			actual = string(info.Type)
		}
		return errWrongIDType{actualType: actual}
	}
	if idSchema.Sensitive() && (info.Secret == nil || *info.Secret) {
		return errSensitiveID{rname}
	}
	return nil
}

// resourceHasComputeID returns true if the resource does not have an "id" field, but
// does have a "ComputeID" func
func resourceHasComputeID(info tfbridge.ProviderInfo, resname string) bool {
	if info.Resources == nil {
		return false
	}

	if info, ok := info.Resources[resname]; ok {
		if _, ok := info.Fields["id"]; !ok && info.ComputeID != nil {
			return true
		}
	}
	return false
}
