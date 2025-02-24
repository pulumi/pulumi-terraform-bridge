// Copyright 2016-2025, Pulumi Corporation.
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

package protov5

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UnimplementedEphemeralResourceServer struct{}

var _ tfprotov5.EphemeralResourceServer = (*UnimplementedEphemeralResourceServer)(nil)

func (UnimplementedEphemeralResourceServer) ValidateEphemeralResourceConfig(
	context.Context,
	*tfprotov5.ValidateEphemeralResourceConfigRequest,
) (*tfprotov5.ValidateEphemeralResourceConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateEphemeralResourceConfig not implemented")
}

func (UnimplementedEphemeralResourceServer) OpenEphemeralResource(
	context.Context,
	*tfprotov5.OpenEphemeralResourceRequest,
) (*tfprotov5.OpenEphemeralResourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method OpenEphemeralResource not implemented")
}

func (UnimplementedEphemeralResourceServer) RenewEphemeralResource(
	context.Context,
	*tfprotov5.RenewEphemeralResourceRequest,
) (*tfprotov5.RenewEphemeralResourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RenewEphemeralResource not implemented")
}

func (UnimplementedEphemeralResourceServer) CloseEphemeralResource(
	context.Context,
	*tfprotov5.CloseEphemeralResourceRequest,
) (*tfprotov5.CloseEphemeralResourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseEphemeralResource not implemented")
}
