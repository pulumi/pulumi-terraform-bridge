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

package schemav6

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	resourceName = "xres"
)

func ResourceSchema(res *schema.Resource) (*tfprotov6.Schema, error) {
	contract.Assertf(res != nil, "res != nil")

	prov := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"xres": res,
		},
	}

	srv, err := tf5to6server.UpgradeServer(context.Background(), func() tfprotov5.ProviderServer {
		return schema.NewGRPCProviderServer(prov)
	})
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, err
	}

	if err := renderDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}

	result, ok := resp.ResourceSchemas[resourceName]
	contract.Assertf(ok, "schemav6.ResourceSchema: no %q resource in GetProviderSchema response", resourceName)

	return result, nil
}

func formatDiagnostic(w io.Writer, d *tfprotov6.Diagnostic) {
	fmt.Fprintf(w, "%v", d.Severity)
	if d.Attribute != nil {
		fmt.Fprintf(w, " at %v", d.Attribute.String())
	}
	fmt.Fprintf(w, ". %s", d.Summary)
	if d.Detail != "" {
		fmt.Fprintf(w, ": %s", d.Detail)
	}
}

func formatDiagnostics(w io.Writer, diags []*tfprotov6.Diagnostic) {
	fmt.Fprintf(w, "%d unexpected diagnostic(s):", len(diags))
	fmt.Fprintln(w)
	for _, d := range diags {
		fmt.Fprintf(w, "- ")
		formatDiagnostic(w, d)
		fmt.Fprintln(w)
	}
}

func renderDiagnostics(diags []*tfprotov6.Diagnostic) error {
	if len(diags) > 0 {
		var buf bytes.Buffer
		formatDiagnostics(&buf, diags)
		return fmt.Errorf("%s", buf.String())
	}
	return nil
}
