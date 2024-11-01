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
	"sync"

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
