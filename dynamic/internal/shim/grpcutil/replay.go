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

package grpcutil

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
)

// LogReplayProvider is a provider that replays logs from a previous run.
type LogReplayProvider struct {
	// LogReplayProvider is safe to copy because methodLogs is a by-reference type.
	// This allows mutating methods on LogReplayProvider to take a by-value copy of
	// LogReplayProvider while sharing the same reference to the underlying data.

	// methodLogs is the set of un-replayed method logs for the provider.
	methodLogs map[string][]grpcLog
}

var _ = tfplugin6.ProviderClient(LogReplayProvider{})

func getMethodFromFullName(fullName string) string {
	splitName := strings.Split(fullName, ".")
	return splitName[len(splitName)-1]
}

// NewLogReplayProvider creates a new LogReplayProvider from the given logs.
// It uses the logs to replay the recorded calls to the provider.
func NewLogReplayProvider(name, version string, logs []byte) LogReplayProvider {
	var grpcLogs []grpcLog
	err := json.Unmarshal(logs, &grpcLogs)
	contract.AssertNoErrorf(err, "failed to unmarshal logs")
	methodLogs := make(map[string][]grpcLog)
	for _, log := range grpcLogs {
		methodName := getMethodFromFullName(log.Name)
		methodLogs[methodName] = append(methodLogs[methodName], log)
	}
	return LogReplayProvider{
		methodLogs: methodLogs,
	}
}

func (p LogReplayProvider) mustPopLog(method string) grpcLog {
	logs, ok := p.methodLogs[method]
	contract.Assertf(ok && len(logs) > 0, "no logs for method %s, logs: %s", method, p.methodLogs)
	// TODO[pulumi/pulumi-terraform-bridge#2571]: Consider a cleverer way of matching multiple logs for the same method.
	log := logs[0]
	logs = logs[1:]
	p.methodLogs[method] = logs
	return log
}

type protoMessage[T any] interface {
	ProtoReflect() protoreflect.Message
	// We need to use a type parameter for the underlying type in order to be able to create it.
	*T
}

func mustUnmarshalLog[Q any, R any, T protoMessage[R]](p LogReplayProvider, methodName string, req T) *Q {
	log := p.mustPopLog(methodName)
	contract.Assertf(
		getMethodFromFullName(log.Name) == methodName,
		"log name %q does not match method name %q", log.Name, methodName)

	// We need to unmarshal the request to compare the fields because stringifying them is unreliable.
	var loggedReq R
	var loggedReqMsg T = &loggedReq
	err := prototext.Unmarshal([]byte(log.Request), loggedReqMsg)
	contract.AssertNoErrorf(err, "failed to unmarshal log request %s", log.Request)

	var checkEqual func(from, to protoreflect.Message)
	checkEqual = func(from, to protoreflect.Message) {
		from.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			expected := to.Get(fd)

			// [protoreflect.Message].String() displays like this:
			//
			//	&{{} [] [] 0x14000410008}
			//
			// If we find two messages that are not equal and use the normal
			// [contract.Assertf] to error, the error message isn't helpful:
			//
			//	panic: fatal: An assertion has failed: field "config" does not match logged field:
			//
			//		&{{} [] [] 0x14000410008} != &{{} [] [] 0x14000410008}
			//
			// To get useful error reporting, we manually recurse.
			if v, ok := v.Interface().(protoreflect.Message); ok {
				if expected, ok := expected.Interface().(protoreflect.Message); ok {
					checkEqual(v, expected)
					return true
				}
			}
			contract.Assertf(v.Equal(expected),
				"field %q does not match logged field %s != %s", fd.FullName(), expected, v)
			return true
		})
	}

	checkEqual(loggedReqMsg.ProtoReflect(), req.ProtoReflect())
	checkEqual(req.ProtoReflect(), loggedReqMsg.ProtoReflect())

	var resp Q
	err = json.Unmarshal([]byte(log.Response), &resp)
	contract.AssertNoErrorf(err, "failed to unmarshal log response %s", log.Response)
	return &resp
}

func (p LogReplayProvider) GetMetadata(
	ctx context.Context, req *tfplugin6.GetMetadata_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetMetadata_Response, error) {
	return mustUnmarshalLog[tfplugin6.GetMetadata_Response](p, "GetMetadata", req), nil
}

func (p LogReplayProvider) GetProviderSchema(
	ctx context.Context, req *tfplugin6.GetProviderSchema_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetProviderSchema_Response, error) {
	return mustUnmarshalLog[tfplugin6.GetProviderSchema_Response](p, "GetProviderSchema", req), nil
}

func (p LogReplayProvider) ValidateProviderConfig(
	ctx context.Context, req *tfplugin6.ValidateProviderConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateProviderConfig_Response, error) {
	return mustUnmarshalLog[tfplugin6.ValidateProviderConfig_Response](p, "ValidateProviderConfig", req), nil
}

func (p LogReplayProvider) ConfigureProvider(
	ctx context.Context, req *tfplugin6.ConfigureProvider_Request, opts ...grpc.CallOption,
) (*tfplugin6.ConfigureProvider_Response, error) {
	req.ProtoMessage()
	return mustUnmarshalLog[tfplugin6.ConfigureProvider_Response](p, "ConfigureProvider", req), nil
}

func (p LogReplayProvider) StopProvider(
	ctx context.Context, req *tfplugin6.StopProvider_Request, opts ...grpc.CallOption,
) (*tfplugin6.StopProvider_Response, error) {
	return mustUnmarshalLog[tfplugin6.StopProvider_Response](p, "StopProvider", req), nil
}

func (p LogReplayProvider) ValidateResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateResourceConfig_Response, error) {
	return mustUnmarshalLog[tfplugin6.ValidateResourceConfig_Response](p, "ValidateResourceConfig", req), nil
}

func (p LogReplayProvider) UpgradeResourceState(
	ctx context.Context, req *tfplugin6.UpgradeResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.UpgradeResourceState_Response, error) {
	return mustUnmarshalLog[tfplugin6.UpgradeResourceState_Response](p, "UpgradeResourceState", req), nil
}

func (p LogReplayProvider) ReadResource(
	ctx context.Context, req *tfplugin6.ReadResource_Request, opts ...grpc.CallOption,
) (*tfplugin6.ReadResource_Response, error) {
	return mustUnmarshalLog[tfplugin6.ReadResource_Response](p, "ReadResource", req), nil
}

func (p LogReplayProvider) PlanResourceChange(
	ctx context.Context, req *tfplugin6.PlanResourceChange_Request, opts ...grpc.CallOption,
) (*tfplugin6.PlanResourceChange_Response, error) {
	return mustUnmarshalLog[tfplugin6.PlanResourceChange_Response](p, "PlanResourceChange", req), nil
}

func (p LogReplayProvider) ApplyResourceChange(
	ctx context.Context, req *tfplugin6.ApplyResourceChange_Request, opts ...grpc.CallOption,
) (*tfplugin6.ApplyResourceChange_Response, error) {
	return mustUnmarshalLog[tfplugin6.ApplyResourceChange_Response](p, "ApplyResourceChange", req), nil
}

func (p LogReplayProvider) ImportResourceState(
	ctx context.Context, req *tfplugin6.ImportResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.ImportResourceState_Response, error) {
	return mustUnmarshalLog[tfplugin6.ImportResourceState_Response](p, "ImportResourceState", req), nil
}

func (p LogReplayProvider) ValidateDataResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateDataResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateDataResourceConfig_Response, error) {
	return mustUnmarshalLog[tfplugin6.ValidateDataResourceConfig_Response](p, "ValidateDataResourceConfig", req), nil
}

func (p LogReplayProvider) MoveResourceState(
	ctx context.Context, req *tfplugin6.MoveResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.MoveResourceState_Response, error) {
	return mustUnmarshalLog[tfplugin6.MoveResourceState_Response](p, "MoveResourceState", req), nil
}

func (p LogReplayProvider) ReadDataSource(
	ctx context.Context, req *tfplugin6.ReadDataSource_Request, opts ...grpc.CallOption,
) (*tfplugin6.ReadDataSource_Response, error) {
	return mustUnmarshalLog[tfplugin6.ReadDataSource_Response](p, "ReadDataSource", req), nil
}

func (p LogReplayProvider) CallFunction(
	ctx context.Context, req *tfplugin6.CallFunction_Request, opts ...grpc.CallOption,
) (*tfplugin6.CallFunction_Response, error) {
	return mustUnmarshalLog[tfplugin6.CallFunction_Response](p, "CallFunction", req), nil
}

func (p LogReplayProvider) GetFunctions(
	ctx context.Context, req *tfplugin6.GetFunctions_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetFunctions_Response, error) {
	return mustUnmarshalLog[tfplugin6.GetFunctions_Response](p, "GetFunctions", req), nil
}

func (p LogReplayProvider) GetResourceIdentitySchemas(
	ctx context.Context, req *tfplugin6.GetResourceIdentitySchemas_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetResourceIdentitySchemas_Response, error) {
	return mustUnmarshalLog[tfplugin6.GetResourceIdentitySchemas_Response](p, "GetResourceIdentitySchemas", req), nil
}

func (p LogReplayProvider) UpgradeResourceIdentity(
	ctx context.Context, req *tfplugin6.UpgradeResourceIdentity_Request, opts ...grpc.CallOption,
) (*tfplugin6.UpgradeResourceIdentity_Response, error) {
	return mustUnmarshalLog[tfplugin6.UpgradeResourceIdentity_Response](p, "UpgradeResourceIdentity", req), nil
}

func (p LogReplayProvider) ValidateEphemeralResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateEphemeralResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateEphemeralResourceConfig_Response, error) {
	return mustUnmarshalLog[tfplugin6.ValidateEphemeralResourceConfig_Response](
		p,
		"ValidateEphemeralResourceConfig",
		req,
	), nil
}

func (p LogReplayProvider) OpenEphemeralResource(
	ctx context.Context, req *tfplugin6.OpenEphemeralResource_Request, opts ...grpc.CallOption,
) (*tfplugin6.OpenEphemeralResource_Response, error) {
	return mustUnmarshalLog[tfplugin6.OpenEphemeralResource_Response](p, "OpenEphemeralResource", req), nil
}

func (p LogReplayProvider) RenewEphemeralResource(
	ctx context.Context, req *tfplugin6.RenewEphemeralResource_Request, opts ...grpc.CallOption,
) (*tfplugin6.RenewEphemeralResource_Response, error) {
	return mustUnmarshalLog[tfplugin6.RenewEphemeralResource_Response](p, "RenewEphemeralResource", req), nil
}

func (p LogReplayProvider) CloseEphemeralResource(
	ctx context.Context, req *tfplugin6.CloseEphemeralResource_Request, opts ...grpc.CallOption,
) (*tfplugin6.CloseEphemeralResource_Response, error) {
	return mustUnmarshalLog[tfplugin6.CloseEphemeralResource_Response](p, "CloseEphemeralResource", req), nil
}

func (p LogReplayProvider) Close() error {
	return nil
}
