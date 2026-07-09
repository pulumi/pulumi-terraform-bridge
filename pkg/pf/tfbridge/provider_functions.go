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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/functions"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
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
	pulumiInfo      *info.Function
	argNames        []string
	variadicArgName string // empty if the function has no variadic parameter

	argsType tftypes.Object
	encoder  convert.Encoder

	resultType    tftypes.Object // the object return type, or the wrapper for a direct return
	objectResult  bool
	decoder       convert.Decoder
	resultUnknown resource.PropertyMap // the result shape when arguments are unknown
}

// functionHandle resolves a Pulumi token to a provider-defined function, reporting false
// if the token does not map to one.
func (p *provider) functionHandle(token tokens.ModuleMember) (functionHandle, bool, error) {
	var tfName string
	var pulumiInfo *info.Function
	for name, v := range p.info.Functions {
		if v.Tok == token {
			tfName, pulumiInfo = name, v
			break
		}
	}
	if pulumiInfo == nil {
		return functionHandle{}, false, nil
	}

	fn, ok := p.schemaOnlyProvider.Functions()[tfName]
	if !ok {
		return functionHandle{}, false, fmt.Errorf(
			"[pf/tfbridge] token %v is mapped to TF function %q, but the provider does not define it", token, tfName)
	}

	fail := func(err error) (functionHandle, bool, error) {
		return functionHandle{}, false, fmt.Errorf("[pf/tfbridge] function %q: %w", tfName, err)
	}

	argNames := functions.ArgumentNames(fn)
	argsSchema, err := functions.ArgumentsSchema(fn, argNames)
	if err != nil {
		return fail(err)
	}
	encoder, err := convert.NewObjectEncoder(convert.ObjectSchema{
		SchemaMap:   argsSchema.SchemaMap,
		SchemaInfos: argsSchema.SchemaInfos,
		Object:      &argsSchema.Type,
	})
	if err != nil {
		return fail(err)
	}

	resultSchema, objectResult, err := functions.ResultSchema(fn, functionResultProperty)
	if err != nil {
		return fail(err)
	}
	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   resultSchema.SchemaMap,
		SchemaInfos: resultSchema.SchemaInfos,
		Object:      &resultSchema.Type,
	})
	if err != nil {
		return fail(err)
	}

	// Precompute the fully-unknown result shape returned for unknown arguments.
	resultUnknown := make(resource.PropertyMap, len(resultSchema.Type.AttributeTypes))
	for attr := range resultSchema.Type.AttributeTypes {
		name := tfbridge.TerraformToPulumiNameV2(attr, resultSchema.SchemaMap, resultSchema.SchemaInfos)
		resultUnknown[resource.PropertyKey(name)] = resource.MakeComputed(resource.NewStringProperty(""))
	}

	h := functionHandle{
		token:         token,
		terraformName: tfName,
		fn:            fn,
		pulumiInfo:    pulumiInfo,
		argNames:      argNames,
		argsType:      argsSchema.Type,
		encoder:       encoder,
		resultType:    resultSchema.Type,
		objectResult:  objectResult,
		decoder:       decoder,
		resultUnknown: resultUnknown,
	}
	if fn.VariadicParameter != nil {
		h.variadicArgName = argNames[len(fn.Parameters)]
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

	// Functions are pure, so an unknown argument makes the whole result unknown; there
	// is nothing to call until the inputs resolve.
	if args.ContainsUnknowns() {
		return h.resultUnknown.Copy(), nil, nil
	}

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
	requireNonNull := func(param shim.FunctionParameter, name string, value resource.PropertyValue) {
		if value.IsNull() && !param.AllowNullValue {
			failures = append(failures, plugin.CheckFailure{
				Property: resource.PropertyKey(name),
				Reason: fmt.Sprintf("function %q requires a non-null value for argument %q",
					h.terraformName, name),
			})
		}
	}
	for i, param := range fn.Parameters {
		requireNonNull(param, h.argNames[i], args[resource.PropertyKey(h.argNames[i])])
	}
	var variadicValues []resource.PropertyValue
	if v := fn.VariadicParameter; v != nil {
		if value, has := args[resource.PropertyKey(h.variadicArgName)]; has && !value.IsNull() {
			if !value.IsArray() {
				return nil, nil, fmt.Errorf("argument %q of function %q must be a list, got %v",
					h.variadicArgName, h.terraformName, value.TypeString())
			}
			variadicValues = value.ArrayValue()
			for _, e := range variadicValues {
				requireNonNull(*v, h.variadicArgName, e)
			}
		}
	}
	if len(failures) > 0 {
		return nil, failures, nil
	}

	// Encode all arguments at once as a synthetic object, then split the object into
	// the positional call arguments, expanding the trailing variadic list.
	encoded, err := convert.EncodePropertyMap(h.encoder, args)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot encode the arguments of function %q: %w", h.terraformName, err)
	}
	var encodedArgs map[string]tftypes.Value
	if err := encoded.As(&encodedArgs); err != nil {
		return nil, nil, fmt.Errorf("cannot encode the arguments of function %q: %w", h.terraformName, err)
	}

	arguments := make([]*tfprotov6.DynamicValue, 0, len(fn.Parameters)+len(variadicValues))
	appendArgument := func(t tftypes.Type, name string, value tftypes.Value) error {
		dv, err := tfprotov6.NewDynamicValue(t, value)
		if err != nil {
			return fmt.Errorf("cannot marshal argument %q of function %q: %w", name, h.terraformName, err)
		}
		arguments = append(arguments, &dv)
		return nil
	}
	for i, param := range fn.Parameters {
		if err := appendArgument(param.Type, h.argNames[i], encodedArgs[h.argNames[i]]); err != nil {
			return nil, nil, err
		}
	}
	if v := fn.VariadicParameter; v != nil && len(variadicValues) > 0 {
		var elements []tftypes.Value
		if err := encodedArgs[h.variadicArgName].As(&elements); err != nil {
			return nil, nil, fmt.Errorf("cannot encode argument %q of function %q: %w",
				h.variadicArgName, h.terraformName, err)
		}
		for _, e := range elements {
			if err := appendArgument(v.Type, h.variadicArgName, e); err != nil {
				return nil, nil, err
			}
		}
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
			if name := h.argNameForCallArgument(*respErr.FunctionArgument); name != "" {
				return nil, []plugin.CheckFailure{{
					Property: resource.PropertyKey(name),
					Reason:   fmt.Sprintf("function %q: %s", h.terraformName, respErr.Text),
				}}, nil
			}
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
	if !h.objectResult {
		// Wrap a direct result into the single-property object the decoder expects.
		resultValue = tftypes.NewValue(h.resultType, map[string]tftypes.Value{
			functionResultProperty: resultValue,
		})
	}
	result, err := convert.DecodePropertyMap(ctx, h.decoder, resultValue)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot decode the result of function %q: %w", h.terraformName, err)
	}
	return result, nil, nil
}

// argNameForCallArgument maps a zero-based CallFunction argument position back to the
// Pulumi argument name. Positions past the declared parameters address expanded variadic
// arguments, which all project to the final list-typed Pulumi argument; when there is no
// variadic parameter such a position is out of range and maps to no argument.
func (h functionHandle) argNameForCallArgument(position int64) string {
	if position >= 0 && position < int64(len(h.fn.Parameters)) {
		return h.argNames[position]
	}
	return h.variadicArgName
}
