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

//go:build generate
// +build generate

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

//go:generate go run generate.go

const (
	oldPkg       = "github.com/opentofu/opentofu"
	newPkg       = "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu"
	protoPkg     = "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
	proto5Pkg    = "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin5"
	opentofuRepo = "https://github.com/opentofu/opentofu.git"
	opentofuVer  = "v1.7.2"
)

type file struct {
	src        string
	dest       string
	transforms []func(string) string
}

func main() {
	remote := fetchRemote()
	fmt.Println(remote)

	for _, f := range files() {
		install(remote, f)
	}
}

func files() []file {

	replacePkg := gofmtReplace(fmt.Sprintf(
		`"%s/internal/configs/configschema" -> "%s/configs/configschema"`,
		oldPkg, newPkg,
	))

	fixupTFPlugin6Ref := gofmtReplace(fmt.Sprintf(
		`"%s" -> "%s"`,
		fmt.Sprintf("%s/internal/tfplugin6", oldPkg),
		protoPkg,
	))

	fixupTFPlugin5Ref := gofmtReplace(fmt.Sprintf(
		`"%s" -> "%s"`,
		fmt.Sprintf("%s/internal/tfplugin5", oldPkg),
		proto5Pkg,
	))

	replaceTfDiagsRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/tfdiags" -> "%s/tfdiags"`,
		oldPkg, newPkg,
	))

	replaceAddrsRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/addrs" -> "%s/addrs"`,
		oldPkg, newPkg,
	))

	replaceHttpClientRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/httpclient" -> "%s/httpclient"`,
		oldPkg, newPkg,
	))

	replaceLoggingRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/logging" -> "%s/logging"`,
		oldPkg, newPkg,
	))

	replaceGetProvidersRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/getproviders" -> "%s/getproviders"`,
		oldPkg, newPkg,
	))

	replaceCopyRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/copy" -> "%s/copy"`,
		oldPkg, newPkg,
	))

	replaceProvidersRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/providers" -> "%s/providers"`,
		oldPkg, newPkg,
	))

	replaceConfigsHcl2ShimRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/configs/hcl2shim" -> "%s/configs/hcl2shim"`,
		oldPkg, newPkg,
	))

	replaceStatesRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/states" -> "%s/states"`,
		oldPkg, newPkg,
	))

	replacePluginConvertRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/plugin/convert" -> "%s/plugin/convert"`,
		oldPkg, newPkg,
	))

	replacePlugin6Ref := gofmtReplace(fmt.Sprintf(
		`"%s/internal/plugin6" -> "%s/plugin6"`,
		oldPkg, newPkg,
	))

	replacePlugin6ConvertRef := gofmtReplace(fmt.Sprintf(
		`"%s/internal/plugin6/convert" -> "%s/plugin6/convert"`,
		oldPkg, newPkg,
	))

	transforms := []func(string) string{
		replacePkg,
		doNotEditWarning,
		fixupCodeTypeError,
		fixupTFPlugin5Ref,
		fixupTFPlugin6Ref,
		replaceTfDiagsRef,
		replaceAddrsRef,
		replaceHttpClientRef,
		replaceLoggingRef,
		replaceGetProvidersRef,
		replaceCopyRef,
		replacePluginConvertRef,
		replacePlugin6ConvertRef,
		replaceProvidersRef,
		replaceStatesRef,
		replaceConfigsHcl2ShimRef,
		replacePlugin6Ref,
	}

	return []file{
		{
			src:  "LICENSE",
			dest: "configs/configschema/LICENSE",
		},
		{
			src:  "LICENSE",
			dest: "plans/objchange/LICENSE",
		},
		{
			src:        "internal/configs/configschema/schema.go",
			dest:       "configs/configschema/schema.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/empty_value.go",
			dest:       "configs/configschema/empty_value.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/implied_type.go",
			dest:       "configs/configschema/implied_type.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/decoder_spec.go",
			dest:       "configs/configschema/decoder_spec.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/path.go",
			dest:       "configs/configschema/path.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/nestingmode_string.go",
			dest:       "configs/configschema/nestingmode_string.go",
			transforms: transforms,
		},
		{
			src:        "internal/plans/objchange/objchange.go",
			dest:       "plans/objchange/objchange.go",
			transforms: append(transforms, patchProposedNewForUnknownBlocks),
		},
		{
			src:        "internal/plans/objchange/plan_valid.go",
			dest:       "plans/objchange/plan_valid.go",
			transforms: transforms,
		},
		// 		{
		// 			src:  "internal/plugin6/convert/schema.go",
		// 			dest: "convert/schema.go",
		// 			transforms: append(transforms, func(s string) string {
		// 				elided :=
		// 					`func ProtoToProviderSchema(s *proto.Schema) providers.Schema {
		// 	return providers.Schema{
		// 		Version: s.Version,
		// 		Block:   ProtoToConfigSchema(s.Block),
		// 	}
		// }`
		// 				s = strings.ReplaceAll(s, elided, "")
		// 				s = strings.ReplaceAll(s, `"github.com/opentofu/opentofu/internal/providers"`, "")
		// 				return s
		// 			}),
		// 		},
		{
			src:        "internal/tfdiags/config_traversals.go",
			dest:       "tfdiags/config_traversals.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/diagnostic_base.go",
			dest:       "tfdiags/diagnostic_base.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/rpc_friendly.go",
			dest:       "tfdiags/rpc_friendly.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/contextual.go",
			dest:       "tfdiags/contextual.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/error.go",
			dest:       "tfdiags/error.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/hcl.go",
			dest:       "tfdiags/hcl.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/source_range.go",
			dest:       "tfdiags/source_range.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/diagnostic.go",
			dest:       "tfdiags/diagnostic.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/diagnostics.go",
			dest:       "tfdiags/diagnostics.go",
			transforms: transforms,
		},
		{
			src:        "internal/tfdiags/sourceless.go",
			dest:       "tfdiags/sourceless.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/provider.go",
			dest:       "addrs/provider.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/targetable.go",
			dest:       "addrs/targetable.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/resource.go",
			dest:       "addrs/resource.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/module_call.go",
			dest:       "addrs/module_call.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/output_value.go",
			dest:       "addrs/output_value.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/check_rule.go",
			dest:       "addrs/check_rule.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/checkable.go",
			dest:       "addrs/checkable.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/check.go",
			dest:       "addrs/check.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/referenceable.go",
			dest:       "addrs/referenceable.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/unique_key.go",
			dest:       "addrs/unique_key.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/instance_key.go",
			dest:       "addrs/instance_key.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/input_variable.go",
			dest:       "addrs/input_variable.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/parse_target.go",
			dest:       "addrs/parse_target.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/module_instance.go",
			dest:       "addrs/module_instance.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/module.go",
			dest:       "addrs/module.go",
			transforms: transforms,
		},
		{
			src:        "internal/addrs/resource.go",
			dest:       "addrs/resource.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/source.go",
			dest:       "getproviders/source.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/types.go",
			dest:       "getproviders/types.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/hash.go",
			dest:       "getproviders/hash.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/errors.go",
			dest:       "getproviders/errors.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/package_authentication.go",
			dest:       "getproviders/package_authentication.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/registry_client.go",
			dest:       "getproviders/registry_client.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/registry_source.go",
			dest:       "getproviders/registry_source.go",
			transforms: transforms,
		},
		{
			src:        "internal/getproviders/filesystem_search.go",
			dest:       "getproviders/filesystem_search.go",
			transforms: transforms,
		},
		{
			src:        "internal/httpclient/client.go",
			dest:       "httpclient/client.go",
			transforms: transforms,
		},
		{
			src:        "internal/httpclient/useragent.go",
			dest:       "httpclient/useragent.go",
			transforms: transforms,
		},
		{
			src:        "internal/logging/logging.go",
			dest:       "logging/logging.go",
			transforms: transforms,
		},
		{
			src:        "internal/logging/panic.go",
			dest:       "logging/panic.go",
			transforms: transforms,
		},
		{
			src:        "internal/providercache/cached_provider.go",
			dest:       "providercache/cached_provider.go",
			transforms: transforms,
		},
		{
			src:        "internal/providercache/dir.go",
			dest:       "providercache/dir.go",
			transforms: transforms,
		},
		{
			src:        "internal/providercache/dir_modify.go",
			dest:       "providercache/dir_modify.go",
			transforms: transforms,
		},
		{
			src:        "internal/providercache/package_install.go",
			dest:       "providercache/package_install.go",
			transforms: transforms,
		},
		{
			src:        "internal/copy/copy_dir.go",
			dest:       "copy/copy_dir.go",
			transforms: transforms,
		},
		{
			src:        "internal/copy/copy_file.go",
			dest:       "copy/copy_file.go",
			transforms: transforms,
		},
		{
			src:  "internal/plugin/plugin.go",
			dest: "plugin/plugin.go",
			transforms: append(transforms, func(s string) string {
				// imp := `"github.com/opentofu/opentofu/internal/plugin6"`
				// s = strings.ReplaceAll(s, imp, "")
				code := `"provisioner": &GRPCProvisionerPlugin{},`
				s = strings.ReplaceAll(s, code, "")
				return s
			}),
		},
		{
			src:  "internal/plugin/serve.go",
			dest: "plugin/serve.go",
			transforms: append(transforms, func(s string) string {
				code := `if opts.GRPCProvisionerFunc != nil {
			plugins[5]["provisioner"] = &GRPCProvisionerPlugin{
				GRPCProvisioner: opts.GRPCProvisionerFunc,
			}
		}`
				s = strings.ReplaceAll(s, code, "")
				return s

			}),
		},
		{
			src:        "internal/plugin/grpc_error.go",
			dest:       "plugin/grpc_error.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin/grpc_provider.go",
			dest:       "plugin/grpc_provider.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin/convert/diagnostics.go",
			dest:       "plugin/convert/diagnostics.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin/convert/schema.go",
			dest:       "plugin/convert/schema.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin/convert/function.go",
			dest:       "plugin/convert/function.go",
			transforms: transforms,
		},
		{
			src:        "internal/providers/schemas.go",
			dest:       "providers/schemas.go",
			transforms: transforms,
		},
		{
			src:        "internal/providers/schema_cache.go",
			dest:       "providers/schema_cache.go",
			transforms: transforms,
		},
		{
			src:  "internal/providers/provider.go",
			dest: "providers/provider.go",
			transforms: append(transforms, func(s string) string {
				imp := `"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/states"`
				s = strings.ReplaceAll(s, imp, "")
				code := `func (ir ImportedResource) AsInstanceObject() *states.ResourceInstanceObject {
	return &states.ResourceInstanceObject{
		Status:  states.ObjectReady,
		Value:   ir.State,
		Private: ir.Private,
	}
}`
				s = strings.ReplaceAll(s, code, "")
				return s
			}),
		},
		{
			src:        "internal/configs/hcl2shim/flatmap.go",
			dest:       "configs/hcl2shim/flatmap.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/hcl2shim/values.go",
			dest:       "configs/hcl2shim/values.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin6/grpc_provider.go",
			dest:       "plugin6/grpc_provider.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin6/grpc_error.go",
			dest:       "plugin6/grpc_error.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin6/convert/diagnostics.go",
			dest:       "plugin6/convert/diagnostics.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin6/convert/schema.go",
			dest:       "plugin6/convert/schema.go",
			transforms: transforms,
		},
		{
			src:        "internal/plugin6/convert/function.go",
			dest:       "plugin6/convert/function.go",
			transforms: transforms,
		},
	}
}

func install(remote string, f file) {
	srcPath := filepath.Join(remote, filepath.Join(strings.Split(f.src, "/")...))
	code, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range f.transforms {
		code = []byte(t(string(code)))
	}
	destPath := filepath.Join(strings.Split(f.dest, "/")...)
	ensureDirFor(destPath)
	if err := os.WriteFile(destPath, code, os.ModePerm); err != nil {
		log.Fatal(err)
	}
}

func ensureDirFor(path string) {
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

func fetchRemote() string {
	tmp := os.TempDir()
	dir := filepath.Join(tmp, "opentofu-"+opentofuVer)
	stat, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	if os.IsNotExist(err) || !stat.IsDir() {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command("git", "clone", "-b", opentofuVer, opentofuRepo, dir)
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
	return dir
}

func gofmtReplace(spec string) func(string) string {
	return func(code string) string {
		t, err := os.CreateTemp("", "gofmt*.go")
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(t.Name())
		if err := os.WriteFile(t.Name(), []byte(code), os.ModePerm); err != nil {
			log.Fatal(err)
		}
		var stdout bytes.Buffer
		cmd := exec.Command("gofmt", "-r", spec, t.Name())
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
		return stdout.String()
	}
}

func doNotEditWarning(code string) string {
	return "// Code copied from " + opentofuRepo + " by go generate; DO NOT EDIT.\n" + code
}

func fixupCodeTypeError(code string) string {
	before := `panic(fmt.Sprintf("unsupported block nesting mode %s"`
	after := `panic(fmt.Sprintf("unsupported block nesting mode %v"`
	return strings.ReplaceAll(code, before, after)
}

// This patch introduces a change in behavior for the vendored objchange.ProposedNew algorithm. Before the change,
// planning a block change where config is entirely unknown used to pick the prior state. After the change it picks the
// unknown. This is especially interesting when planning set-nested blocks, as when the algorithm fails to find a
// matching set element in priorState it will send prior=null instead, and proceed to substitute null with an empty
// value matching the block structure. Without the patch, this empty value will be selected over the unknown and
// surfaced to Pulumi users, which is confusing.
//
// See TestUnknowns test suite and the "unknown for set block prop" test case.
//
// TODO[pulumi/pulumi-terraform-bridge#2247] revisit this patch.
func patchProposedNewForUnknownBlocks(goCode string) string {
	oldCode := `func proposedNew(schema *configschema.Block, prior, config cty.Value) cty.Value {
	if config.IsNull() || !config.IsKnown() {`

	newCode := `func proposedNew(schema *configschema.Block, prior, config cty.Value) cty.Value {
	if !config.IsKnown() {
		return config
	}
	if config.IsNull() {`
	updatedGoCode := strings.Replace(goCode, oldCode, newCode, 1)
	contract.Assertf(updatedGoCode != oldCode, "patchProposedNewForUnknownBlocks failed to apply")
	return updatedGoCode
}
