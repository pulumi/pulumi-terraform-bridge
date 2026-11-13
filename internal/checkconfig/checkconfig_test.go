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

package checkconfig

import (
	"context"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestCheckConfig2(t *testing.T) {
	t.Parallel()

	mkStruct := func(m map[string]any) *structpb.Struct {
		s, err := structpb.NewStruct(m)
		require.NoError(t, err)
		return s
	}

	req := &pulumirpc.CheckConfigRequest{
		Urn:        "urn:pulumi:dev::proj::pulumi:providers:pkg::default",
		Olds:       mkStruct(map[string]any{"region": "us-east-1"}),
		News:       mkStruct(map[string]any{"region": "us-west-2"}),
		RandomSeed: []byte("seed"),
		Name:       "default",
		Type:       "pulumi:providers:pkg",
		OldOutputs: mkStruct(map[string]any{"version": "1.0.0"}),
	}

	t.Run("forwards the request and converts the response", func(t *testing.T) {
		t.Parallel()

		var got *pulumirpc.CheckRequest
		resp, err := CheckConfig2(t.Context(), req,
			func(_ context.Context, r *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
				got = r
				return &pulumirpc.CheckResponse{
					Inputs:   r.GetNews(),
					Failures: []*pulumirpc.CheckFailure{{Property: "region", Reason: "bad region"}},
				}, nil
			})
		require.NoError(t, err)

		assert.True(t, proto.Equal(&pulumirpc.CheckRequest{
			Urn:        req.Urn,
			Olds:       req.Olds,
			News:       req.News,
			RandomSeed: req.RandomSeed,
			Name:       req.Name,
			Type:       req.Type,
			OldOutputs: req.OldOutputs,
		}, got), "unexpected CheckRequest: %v", got)

		assert.True(t, proto.Equal(&pulumirpc.CheckConfigResponse{
			Inputs:   req.News,
			Failures: []*pulumirpc.CheckConfigFailure{{Property: "region", Reason: "bad region"}},
		}, resp), "unexpected CheckConfigResponse: %v", resp)
	})

	t.Run("a request decodes the same as either type", func(t *testing.T) {
		t.Parallel()

		bytes, err := proto.Marshal(req)
		require.NoError(t, err)
		var decoded pulumirpc.CheckRequest
		require.NoError(t, proto.Unmarshal(bytes, &decoded))

		var got *pulumirpc.CheckRequest
		_, err = CheckConfig2(t.Context(), req,
			func(_ context.Context, r *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
				got = r
				return &pulumirpc.CheckResponse{}, nil
			})
		require.NoError(t, err)
		assert.True(t, proto.Equal(&decoded, got), "expected %v, got %v", &decoded, got)
	})

	t.Run("passes errors through", func(t *testing.T) {
		t.Parallel()

		resp, err := CheckConfig2(t.Context(), req,
			func(context.Context, *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
				return nil, status.Error(codes.Unimplemented, "CheckConfig is not implemented")
			})
		assert.Nil(t, resp)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})
}

func TestCheckConfig(t *testing.T) {
	t.Parallel()

	news, err := structpb.NewStruct(map[string]any{"region": "us-west-2"})
	require.NoError(t, err)
	req := &pulumirpc.CheckRequest{
		Urn:        "urn:pulumi:dev::proj::pulumi:providers:pkg::default",
		News:       news,
		RandomSeed: []byte("seed"),
		Name:       "default",
		Type:       "pulumi:providers:pkg",
	}

	t.Run("forwards the request and drops remediations", func(t *testing.T) {
		t.Parallel()

		var got *pulumirpc.CheckConfigRequest
		resp, err := CheckConfig(t.Context(), req,
			func(_ context.Context, r *pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
				got = r
				return &pulumirpc.CheckConfigResponse{
					Inputs: r.GetNews(),
					Failures: []*pulumirpc.CheckConfigFailure{{
						Property: "region",
						Reason:   "bad region",
						Remediations: []*pulumirpc.CommandRemediation{{
							Description: "Sign in",
							Command:     []string{"cloud", "login"},
						}},
					}},
				}, nil
			})
		require.NoError(t, err)

		assert.True(t, proto.Equal(&pulumirpc.CheckConfigRequest{
			Urn:        req.Urn,
			News:       req.News,
			RandomSeed: req.RandomSeed,
			Name:       req.Name,
			Type:       req.Type,
		}, got), "unexpected CheckConfigRequest: %v", got)

		assert.True(t, proto.Equal(&pulumirpc.CheckResponse{
			Inputs:   req.News,
			Failures: []*pulumirpc.CheckFailure{{Property: "region", Reason: "bad region"}},
		}, resp), "unexpected CheckResponse: %v", resp)
	})

	t.Run("passes errors through", func(t *testing.T) {
		t.Parallel()

		resp, err := CheckConfig(t.Context(), req,
			func(context.Context, *pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
				return nil, status.Error(codes.Internal, "boom")
			})
		assert.Nil(t, resp)
		assert.Equal(t, codes.Internal, status.Code(err))
	})
}
