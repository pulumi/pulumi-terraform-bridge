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

package logging

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/hashicorp/terraform-plugin-log/tfsdklog"
)

// The keys from github.com/hashicorp/terraform-plugin-log are internal and cannot be linked in directly; use reflect
// trickery to recover them.
var providerKey, providerOptionsKey, sdkKey, sdkOptionsKey any

func init() {
	k := recoverKeys()
	providerKey = k.provider
	providerOptionsKey = k.providerOptions
	sdkKey = k.sdk
	sdkOptionsKey = k.sdkOptions
}

type keys struct {
	provider        any
	providerOptions any
	sdk             any
	sdkOptions      any
}

type valueCtx struct {
	context.Context
	key, val any
}

func field(v reflect.Value, fieldIndex int) interface{} {
	f := v.Field(fieldIndex)
	f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	return f.Interface()
}

func reflectValueCtx(ctx context.Context) *valueCtx {
	v := reflect.ValueOf(ctx).Elem()
	context := field(v, 0).(context.Context)
	key := field(v, 1)
	value := field(v, 2)
	return &valueCtx{context, key, value}
}

func recoverKeys() keys {
	ctx0 := tfsdklog.NewRootSDKLogger(context.Background())
	ctx1 := reflectValueCtx(ctx0)
	ctx2 := reflectValueCtx(ctx1.Context)
	ctx3 := tfsdklog.NewRootProviderLogger(context.Background())
	ctx4 := reflectValueCtx(ctx3)
	ctx5 := reflectValueCtx(ctx4.Context)
	result := keys{}
	for _, key := range []interface{}{ctx1.key, ctx2.key, ctx4.key, ctx5.key} {
		switch fmt.Sprintf("%s", key) {
		case "sdk-options":
			result.sdkOptions = key
		case "sdk":
			result.sdk = key
		case "provider":
			result.provider = key
		case "provider-options":
			result.providerOptions = key
		}
	}
	return result
}
