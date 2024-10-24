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
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/opentofu/opentofu/internal/tfplugin6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc"
)

var (
	pulumiTFDebugGRPCLogs = os.Getenv("PULUMI_TF_DEBUG_GRPC")
	grpcLogLock           sync.Mutex
	grpcLogFileTarget     *os.File
)

type grpcLog struct {
	Name     string   `json:"name"`
	Call     string   `json:"call"`
	Request  string   `json:"request"`
	Response string   `json:"response"`
	Error    []string `json:"error,omitempty"`
}

func writeGRPCLog(log grpcLog) error {
	if pulumiTFDebugGRPCLogs == "" {
		return nil
	}
	grpcLogLock.Lock()
	defer grpcLogLock.Unlock()

	if grpcLogFileTarget == nil {
		var err error
		if _, err = os.Stat(pulumiTFDebugGRPCLogs); os.IsNotExist(err) {
			grpcLogFileTarget, err = os.Create(pulumiTFDebugGRPCLogs)
			if err != nil {
				return err
			}
		} else {
			grpcLogFileTarget, err = os.OpenFile(pulumiTFDebugGRPCLogs, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
		}
	}

	b, err := json.Marshal(log)
	contract.AssertNoErrorf(err, "%T should always marshal", log)
	_, err = grpcLogFileTarget.Write(append(b, ',', '\n'))
	if err != nil {
		return err
	}
	return grpcLogFileTarget.Sync()
}

func Translate[
	In, Out fmt.Stringer,
	Final any,
	Call func(context.Context, In, ...grpc.CallOption) (Out, error),
	MapResult func(Out) Final,
](
	ctx context.Context,
	call Call,
	i In,
	m MapResult, opts ...grpc.CallOption,
) (_ Final, err error) {
	var log grpcLog
	if pulumiTFDebugGRPCLogs != "" {
		log.Request = i.String()
		if pc, line, num, ok := runtime.Caller(2); ok {
			log.Call = fmt.Sprintf("%s:%d", line, num)
			log.Name = runtime.FuncForPC(pc).Name()
		}
		defer func() {
			e := writeGRPCLog(log)
			if err == nil {
				err = e
			}
		}()
	}
	v, err := call(ctx, i)
	if err != nil {
		var tmp Final
		log.Error = []string{err.Error()}
		return tmp, err
	}
	if pulumiTFDebugGRPCLogs != "" {
		jsonOut, jsonErr := json.Marshal(v)
		if jsonErr != nil {
			err = jsonErr
		}
		log.Response = string(jsonOut)
	}
	return m(v), nil
}

type LogReplayProvider struct {
	methodLogs map[string][]grpcLog
}

var _ = tfplugin6.ProviderClient(LogReplayProvider{})

func getMethodFromFullName(fullName string) string {
	splitName := strings.Split(fullName, ".")
	return splitName[len(splitName)-1]
}

func NewLogReplayProvider(name, version, logs string) LogReplayProvider {
	grpcLogs := make([]grpcLog, 0)
	err := json.Unmarshal([]byte(logs), &grpcLogs)
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
	contract.Assertf(ok, "no logs for method %s, logs: %s", method, p.methodLogs)
	log := logs[0]
	logs = logs[1:]
	p.methodLogs[method] = logs
	return log
}

func mustUnmarshalLog[T fmt.Stringer, Q any](log grpcLog, methodName string, req *T, resp *Q) {
	contract.Assertf(
		getMethodFromFullName(log.Name) == methodName,
		"log name %s does not match method name %s", log.Name, methodName)

	contract.Assertf(req != nil, "request is nil")
	reqString := (*req).String()
	contract.Assertf(
		reqString == log.Request, "request %s does not match log request %s", reqString, log.Request)

	err := json.Unmarshal([]byte(log.Response), resp)
	contract.AssertNoErrorf(err, "failed to unmarshal log response %s", log.Response)
}

func (p LogReplayProvider) GetMetadata(
	ctx context.Context, req *tfplugin6.GetMetadata_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetMetadata_Response, error) {
	methodName := "GetMetadata"
	var resp tfplugin6.GetMetadata_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) GetProviderSchema(
	ctx context.Context, req *tfplugin6.GetProviderSchema_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetProviderSchema_Response, error) {
	methodName := "GetProviderSchema"
	var resp tfplugin6.GetProviderSchema_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ValidateProviderConfig(
	ctx context.Context, req *tfplugin6.ValidateProviderConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateProviderConfig_Response, error) {
	methodName := "ValidateProviderConfig"
	var resp tfplugin6.ValidateProviderConfig_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ConfigureProvider(
	ctx context.Context, req *tfplugin6.ConfigureProvider_Request, opts ...grpc.CallOption,
) (*tfplugin6.ConfigureProvider_Response, error) {
	methodName := "ConfigureProvider"
	var resp tfplugin6.ConfigureProvider_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) StopProvider(
	ctx context.Context, req *tfplugin6.StopProvider_Request, opts ...grpc.CallOption,
) (*tfplugin6.StopProvider_Response, error) {
	methodName := "StopProvider"
	var resp tfplugin6.StopProvider_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ValidateResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateResourceConfig_Response, error) {
	methodName := "ValidateResourceConfig"
	var resp tfplugin6.ValidateResourceConfig_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) UpgradeResourceState(
	ctx context.Context, req *tfplugin6.UpgradeResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.UpgradeResourceState_Response, error) {
	methodName := "UpgradeResourceState"
	var resp tfplugin6.UpgradeResourceState_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ReadResource(
	ctx context.Context, req *tfplugin6.ReadResource_Request, opts ...grpc.CallOption,
) (*tfplugin6.ReadResource_Response, error) {
	methodName := "ReadResource"
	var resp tfplugin6.ReadResource_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) PlanResourceChange(
	ctx context.Context, req *tfplugin6.PlanResourceChange_Request, opts ...grpc.CallOption,
) (*tfplugin6.PlanResourceChange_Response, error) {
	methodName := "PlanResourceChange"
	var resp tfplugin6.PlanResourceChange_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ApplyResourceChange(
	ctx context.Context, req *tfplugin6.ApplyResourceChange_Request, opts ...grpc.CallOption,
) (*tfplugin6.ApplyResourceChange_Response, error) {
	methodName := "ApplyResourceChange"
	var resp tfplugin6.ApplyResourceChange_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ImportResourceState(
	ctx context.Context, req *tfplugin6.ImportResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.ImportResourceState_Response, error) {
	methodName := "ImportResourceState"
	var resp tfplugin6.ImportResourceState_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ValidateDataResourceConfig(
	ctx context.Context, req *tfplugin6.ValidateDataResourceConfig_Request, opts ...grpc.CallOption,
) (*tfplugin6.ValidateDataResourceConfig_Response, error) {
	methodName := "ValidateDataResourceConfig"
	var resp tfplugin6.ValidateDataResourceConfig_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) MoveResourceState(
	ctx context.Context, req *tfplugin6.MoveResourceState_Request, opts ...grpc.CallOption,
) (*tfplugin6.MoveResourceState_Response, error) {
	methodName := "MoveResourceState"
	var resp tfplugin6.MoveResourceState_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) ReadDataSource(
	ctx context.Context, req *tfplugin6.ReadDataSource_Request, opts ...grpc.CallOption,
) (*tfplugin6.ReadDataSource_Response, error) {
	methodName := "ReadDataSource"
	var resp tfplugin6.ReadDataSource_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) CallFunction(
	ctx context.Context, req *tfplugin6.CallFunction_Request, opts ...grpc.CallOption,
) (*tfplugin6.CallFunction_Response, error) {
	methodName := "CallFunction"
	var resp tfplugin6.CallFunction_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) GetFunctions(
	ctx context.Context, req *tfplugin6.GetFunctions_Request, opts ...grpc.CallOption,
) (*tfplugin6.GetFunctions_Response, error) {
	methodName := "GetFunctions"
	var resp tfplugin6.GetFunctions_Response
	mustUnmarshalLog(p.mustPopLog(methodName), methodName, &req, &resp)
	return &resp, nil
}

func (p LogReplayProvider) Close() error {
	return nil
}
