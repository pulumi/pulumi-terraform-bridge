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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func main() {
	err := provider.Main(providerName, func(hc *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return newHclResourceProviderServer(hc), nil
	})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

// Runs terraform init in a given d directory.
//
// Running terraform init will:
//
//	resolve and download modules to .terraform/modules
//	resolve and download providers to .terraform/providers
//	build .terraform.lock.hcl with resolved versions
//
// For the purposes of this code provider binaries will not be needed, so there is is a bit inefficient.
func initTF(d string) error {
	cmd := exec.Command("terraform", "init")

	if _, debug := os.LookupEnv("DEBUG"); debug {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	cmd.Dir = d
	return cmd.Run()
}

// Prepare a folder with TF files to send
func prepareTFWorkspace() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	d := filepath.Join(wd, ".hcl", "temp-workspace")

	err = os.RemoveAll(d)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(d, 0755)
	if err != nil {
		return "", err
	}

	// TODO inputs to TF need to be translated from Pulumi inputs.
	//
	// https://developer.hashicorp.com/terraform/language/syntax/json
	jsonTF := map[string]any{
		"module": map[string]any{
			"vpc": map[string]any{
				// TODO this value needs to be translated from the Parameterize call.
				"source":             "terraform-aws-modules/vpc/aws",
				"name":               "my-vpc",
				"cidr":               "10.0.0.0/16",
				"azs":                []any{"us-east-1a", "us-east-1b"},
				"private_subnets":    []any{"10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"},
				"public_subnets":     []any{"10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"},
				"enable_nat_gateway": true,
				"enable_vpn_gateway": true,
			},
		},
	}

	jsonTFBytes, err := json.MarshalIndent(jsonTF, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filepath.Join(d, "infra.tf.json"), jsonTFBytes, 0755)
	if err != nil {
		return "", err
	}
	return d, nil
}

func runTF(d string, proxies tfProviderProxies, command ...string) error {
	cmd := exec.Command("terraform", command...)
	cmd.Env = os.Environ()
	reattach, err := computeReattachConfig(d, proxies)
	if err != nil {
		return err
	}
	reattachJSON, err := json.Marshal(reattach)
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envTfReattachProviders, string(reattachJSON)))
	if _, debug := os.LookupEnv("DEBUG"); debug {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}
	cmd.Dir = d
	return cmd.Run()
}

func upTF(d string, proxies tfProviderProxies) error {
	return runTF(d, proxies, "apply", "-auto-approve")
}

func planTF(d string, proxies tfProviderProxies) error {
	return runTF(d, proxies, "plan")
}

// Find which providers are used by a TF d workspace directory.
//
// This can use the following command to parse its output:
//
//	terraform providers
func inferTFRequiredProviders(string) (map[string]struct{}, error) {
	// TODO assuming just AWS provider is needed for now.
	return map[string]struct{}{"aws": {}}, nil
}

type tfProviderProxies []*tfProviderProxyHandle

func (pps tfProviderProxies) Close() error {
	var errs []error
	for _, pp := range pps {
		if err := pp.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func startTFProviderProxies(requiredProviders map[string]struct{}, monitorEndpoint string, dryRun bool) (tfProviderProxies, error) {
	res := tfProviderProxies(nil)
	for p := range requiredProviders {
		pp, err := startTFProviderProxy(p, monitorEndpoint, dryRun)
		if err != nil {
			return nil, err
		}
		res = append(res, pp)
	}
	return res, nil
}

// Build a TF_REATTACH_PROVIDERS configuration for a d TF workspace directory.
//
// This will redirect running operations against these providers to Pulumi proxies.
//
// Inferring which providers are needed
// we use debug functionality to make these providers.
// then we run terraform plan
func computeReattachConfig(_ string, proxies tfProviderProxies) (map[string]any, error) {
	cfg := map[string]any{}
	for _, proxy := range proxies {
		cfg[fmt.Sprintf("registry.terraform.io/hashicorp/%s", proxy.ProviderName)] = proxy.computeReattachConfig()
	}
	return cfg, nil
}

func inferPulumiSchemaForModule(pargs *ParameterizeArgs) (*schema.PackageSpec, error) {
	if pargs.TFModuleRef == "terraform-aws-modules/vpc/aws" && pargs.TFModuleVersion == "5.16.0" {
		return &schema.PackageSpec{
			Name:    providerName,
			Version: providerVersion,
			Resources: map[string]schema.ResourceSpec{
				"hcl:index:VpcAws": {
					InputProperties: map[string]schema.PropertySpec{
						"cidr": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"defaultVpcId": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					IsComponent: true,
				},
			},
			Language: map[string]schema.RawMessage{
				"nodejs": schema.RawMessage(`{"respectSchemaVersion": true}`),
			},
			Parameterization: pargs.ToParameterizationSpec(),
		}, nil
	}
	return nil, fmt.Errorf("Cannot infer Pulumi PackageSpec for TF module %q at version %q",
		pargs.TFModuleRef,
		pargs.TFModuleVersion)
}
