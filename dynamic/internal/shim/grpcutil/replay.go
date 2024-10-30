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
	"fmt"
	"strings"

	"github.com/opentofu/opentofu/internal/tfplugin6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc"
)

// LogReplayProvider is a provider that replays logs from a previous run.
type LogReplayProvider struct {
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
	log := logs[0]
	logs = logs[1:]
	p.methodLogs[method] = logs
	return log
}

func mustUnmarshalLog[Q any, T fmt.Stringer](log grpcLog, methodName string, req T) *Q {
	contract.Assertf(
		getMethodFromFullName(log.Name) == methodName,
		"log name %s does not match method name %s", log.Name, methodName)

	reqString := (req).String()
	contract.Assertf(
		reqString == log.Request, "request %s does not match log request %s", reqString, log.Request)

	var resp Q
	err := json.Unmarshal([]byte(log.Response), &resp)
	contract.AssertNoErrorf(err, "failed to unmarshal log response %s", log.Response)
	return &resp
}

func (p LogReplayProvider) GetMetadata(
	ctx context.Context, req *tfplugin6.GetMetadata_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetMetadata_Response, error) {
	methodName := "GetMetadata"
	return mustUnmarshalLog[tfplugin6.GetMetadata_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) GetProviderSchema(
	ctx context.Context, req *tfplugin6.GetProviderSchema_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetProviderSchema_Response, error) {
	methodName := "GetProviderSchema"
	return mustUnmarshalLog[tfplugin6.GetProviderSchema_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ValidateProviderConfig(
	ctx context.Context, req *tfplugin6.ValidateProviderConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateProviderConfig_Response, error) {
	methodName := "ValidateProviderConfig"
	return mustUnmarshalLog[tfplugin6.ValidateProviderConfig_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ConfigureProvider(
	ctx context.Context, req *tfplugin6.ConfigureProvider_Request, opts ...grpc.CallOption,
) (*tfplugin6.ConfigureProvider_Response, error) {
	methodName := "ConfigureProvider"
	return mustUnmarshalLog[tfplugin6.ConfigureProvider_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) StopProvider(
	ctx context.Context, req *tfplugin6.StopProvider_Request, opts ...grpc.CallOption,
) (*tfplugin6.StopProvider_Response, error) {
	methodName := "StopProvider"
	return mustUnmarshalLog[tfplugin6.StopProvider_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ValidateResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateResourceConfig_Response, error) {
	methodName := "ValidateResourceConfig"
	return mustUnmarshalLog[tfplugin6.ValidateResourceConfig_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) UpgradeResourceState(
	ctx context.Context, req *tfplugin6.UpgradeResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.UpgradeResourceState_Response, error) {
	methodName := "UpgradeResourceState"
	return mustUnmarshalLog[tfplugin6.UpgradeResourceState_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ReadResource(
	ctx context.Context, req *tfplugin6.ReadResource_Request, opts ...grpc.CallOption,
) (*tfplugin6.ReadResource_Response, error) {
	methodName := "ReadResource"
	return mustUnmarshalLog[tfplugin6.ReadResource_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) PlanResourceChange(
	ctx context.Context, req *tfplugin6.PlanResourceChange_Request, opts ...grpc.CallOption,
) (*tfplugin6.PlanResourceChange_Response, error) {
	methodName := "PlanResourceChange"
	return mustUnmarshalLog[tfplugin6.PlanResourceChange_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ApplyResourceChange(
	ctx context.Context, req *tfplugin6.ApplyResourceChange_Request, opts ...grpc.CallOption,
) (*tfplugin6.ApplyResourceChange_Response, error) {
	methodName := "ApplyResourceChange"
	return mustUnmarshalLog[tfplugin6.ApplyResourceChange_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ImportResourceState(
	ctx context.Context, req *tfplugin6.ImportResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.ImportResourceState_Response, error) {
	methodName := "ImportResourceState"
	return mustUnmarshalLog[tfplugin6.ImportResourceState_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ValidateDataResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateDataResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateDataResourceConfig_Response, error) {
	methodName := "ValidateDataResourceConfig"
	return mustUnmarshalLog[tfplugin6.ValidateDataResourceConfig_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) MoveResourceState(
	ctx context.Context, req *tfplugin6.MoveResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.MoveResourceState_Response, error) {
	methodName := "MoveResourceState"
	return mustUnmarshalLog[tfplugin6.MoveResourceState_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) ReadDataSource(
	ctx context.Context, req *tfplugin6.ReadDataSource_Request, opts ...grpc.CallOption,
) (*tfplugin6.ReadDataSource_Response, error) {
	methodName := "ReadDataSource"
	return mustUnmarshalLog[tfplugin6.ReadDataSource_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) CallFunction(
	ctx context.Context, req *tfplugin6.CallFunction_Request, opts ...grpc.CallOption,
) (*tfplugin6.CallFunction_Response, error) {
	methodName := "CallFunction"
	return mustUnmarshalLog[tfplugin6.CallFunction_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) GetFunctions(
	ctx context.Context, req *tfplugin6.GetFunctions_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetFunctions_Response, error) {
	methodName := "GetFunctions"
	return mustUnmarshalLog[tfplugin6.GetFunctions_Response](p.mustPopLog(methodName), methodName, req), nil
}

func (p LogReplayProvider) Close() error {
	return nil
}
