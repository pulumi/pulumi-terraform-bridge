// Copyright 2016-2026, Pulumi Corporation.
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

package tfbridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/functions"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// The single-property wire form of a non-object function result. SDKs unwrap the sole
// property of the returned map (e.g. invokeSingle in Node.js), so the key is
// conventional only.
const functionResultProperty = "result"

type functionHandle struct {
	token           tokens.ModuleMember
	terraformName   string
	fn              shim.Function
	argNames        []string
	objectResult    bool
	pulumiInfo      *tfbridge.FunctionInfo
	variadicArgName string // empty if the function has no variadic parameter
}

// functionHandle resolves a Pulumi token to a provider-defined function, reporting false
// if the token does not map to one.
func (p *provider) functionHandle(token tokens.ModuleMember) (functionHandle, bool, error) {
	var tfName string
	var info *tfbridge.FunctionInfo
	for name, v := range p.info.Functions {
		if v.Tok == token {
			tfName, info = name, v
			break
		}
	}
	if info == nil {
		return functionHandle{}, false, nil
	}

	fn, ok := p.schemaOnlyProvider.Functions()[tfName]
	if !ok {
		return functionHandle{}, false, fmt.Errorf(
			"[pf/tfbridge] token %v is mapped to TF function %q, but the provider does not define it", token, tfName)
	}

	argNames := functions.ArgumentNames(fn)
	h := functionHandle{
		token:         token,
		terraformName: tfName,
		fn:            fn,
		argNames:      argNames,
		pulumiInfo:    info,
	}
	if fn.VariadicParameter != nil {
		h.variadicArgName = argNames[len(fn.Parameters)]
	}
	if fn.Return != nil {
		_, h.objectResult = fn.Return.(tftypes.Object)
	}
	return h, true, nil
}

// callFunction implements Pulumi invokes for provider-defined functions by calling the
// Terraform CallFunction RPC. Functions are pure computations: the call does not require
// the provider to be configured.
func (p *provider) callFunction(ctx context.Context, h functionHandle,
	args resource.PropertyMap,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	fn := h.fn

	// Secrets do not survive the invoke protocol; strip them like the data source path
	// does.
	args = propertyvalue.RemoveSecrets(resource.NewObjectProperty(args)).ObjectValue()

	known := make(map[resource.PropertyKey]bool, len(h.argNames))
	for _, name := range h.argNames {
		known[resource.PropertyKey(name)] = true
	}
	var failures []plugin.CheckFailure
	for key := range args {
		if !known[key] {
			failures = append(failures, plugin.CheckFailure{
				Property: key,
				Reason:   fmt.Sprintf("unexpected argument for function %q", h.terraformName),
			})
		}
	}
	if len(failures) > 0 {
		return nil, failures, nil
	}

	arguments := make([]*tfprotov6.DynamicValue, 0, len(fn.Parameters)+1)
	appendArgument := func(param shim.FunctionParameter, name string, value resource.PropertyValue) error {
		if value.IsNull() && !param.AllowNullValue {
			failures = append(failures, plugin.CheckFailure{
				Property: resource.PropertyKey(name),
				Reason: fmt.Sprintf("function %q requires a non-null value for argument %q",
					h.terraformName, name),
			})
			return nil
		}
		v, err := functions.EncodeValue(param.Type, value)
		if err != nil {
			return fmt.Errorf("cannot encode argument %q of function %q: %w", name, h.terraformName, err)
		}
		dv, err := tfprotov6.NewDynamicValue(param.Type, v)
		if err != nil {
			return fmt.Errorf("cannot marshal argument %q of function %q: %w", name, h.terraformName, err)
		}
		arguments = append(arguments, &dv)
		return nil
	}

	for i, param := range fn.Parameters {
		name := h.argNames[i]
		if err := appendArgument(param, name, args[resource.PropertyKey(name)]); err != nil {
			return nil, nil, err
		}
	}
	if v := fn.VariadicParameter; v != nil {
		// The Pulumi projection packs the trailing variadic arguments into a final
		// list-typed argument; unpack it into individual call arguments.
		name := h.variadicArgName
		if value, has := args[resource.PropertyKey(name)]; has && !value.IsNull() {
			value = propertyvalue.RemoveSecrets(value)
			if !value.IsArray() {
				return nil, nil, fmt.Errorf("argument %q of function %q must be a list, got %v",
					name, h.terraformName, value.TypeString())
			}
			for _, e := range value.ArrayValue() {
				if err := appendArgument(*v, name, e); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	if len(failures) > 0 {
		return nil, failures, nil
	}

	resp, err := p.tfServer.CallFunction(ctx, &tfprotov6.CallFunctionRequest{
		Name:      h.terraformName,
		Arguments: arguments,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error calling CallFunction for %q: %w", h.terraformName, err)
	}
	if respErr := resp.Error; respErr != nil {
		if respErr.FunctionArgument != nil {
			name := h.argNameForCallArgument(*respErr.FunctionArgument)
			return nil, []plugin.CheckFailure{{
				Property: resource.PropertyKey(name),
				Reason:   fmt.Sprintf("function %q: %s", h.terraformName, respErr.Text),
			}}, nil
		}
		return nil, nil, fmt.Errorf("function %q returned an error: %s", h.terraformName, respErr.Text)
	}
	if resp.Result == nil {
		return nil, nil, fmt.Errorf("function %q returned no result", h.terraformName)
	}

	resultValue, err := resp.Result.Unmarshal(fn.Return)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot unmarshal the result of function %q: %w", h.terraformName, err)
	}
	result, err := functions.DecodeValue(fn.Return, resultValue)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot decode the result of function %q: %w", h.terraformName, err)
	}

	if h.objectResult {
		if !result.IsObject() {
			return nil, nil, fmt.Errorf("function %q returned a non-object result", h.terraformName)
		}
		return result.ObjectValue(), nil, nil
	}
	return resource.PropertyMap{functionResultProperty: result}, nil, nil
}

// argNameForCallArgument maps a zero-based CallFunction argument position back to the
// Pulumi argument name. Positions past the declared parameters address expanded variadic
// arguments, which all project to the final list-typed Pulumi argument.
func (h functionHandle) argNameForCallArgument(position int64) string {
	if position >= 0 && position < int64(len(h.fn.Parameters)) {
		return h.argNames[position]
	}
	if h.variadicArgName != "" {
		return h.variadicArgName
	}
	if len(h.argNames) > 0 {
		return h.argNames[len(h.argNames)-1]
	}
	return ""
}
