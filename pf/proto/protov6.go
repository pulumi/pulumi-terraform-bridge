package proto

import (
	"context"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func New(ctx context.Context, server tfprotov6.ProviderServer) shim.Provider {
	return Provider{
		server: server,
		getSchema: sync.OnceValue(func() *tfprotov6.GetProviderSchemaResponse {
			if server == nil {
				return &tfprotov6.GetProviderSchemaResponse{
					ServerCapabilities: &tfprotov6.ServerCapabilities{},
					Provider: &tfprotov6.Schema{
						Version: 0,
						Block:   &tfprotov6.SchemaBlock{},
					},
					ProviderMeta:      &tfprotov6.Schema{},
					ResourceSchemas:   map[string]*tfprotov6.Schema{},
					DataSourceSchemas: map[string]*tfprotov6.Schema{},
					Functions:         map[string]*tfprotov6.Function{},
				}
			}
			resp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
			if err != nil {
				// TODO: Return error to the user
				panic(err)
			}
			return resp
		}),
	}
}

// TODO: Make internal
type Provider struct {
	// The underlying server.
	server tfprotov6.ProviderServer
	// A cached GetSchema on the underlying server.
	getSchema func() *tfprotov6.GetProviderSchemaResponse
}

// Get access to the underlying sever used in Provide
func (p Provider) Server() tfprotov6.ProviderServer {
	return cachedSchemaProvider{p.server, p.getSchema}
}

type cachedSchemaProvider struct {
	tfprotov6.ProviderServer

	getSchema func() *tfprotov6.GetProviderSchemaResponse
}

func (p cachedSchemaProvider) GetProviderSchema(
	context.Context, *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	return p.getSchema(), nil
}

func (p Provider) Schema() shim.SchemaMap {
	return blockMap{p.getSchema().Provider.Block}
}

func (p Provider) ResourcesMap() shim.ResourceMap {
	return resourceMap(p.getSchema().ResourceSchemas)
}

func (p Provider) DataSourcesMap() shim.ResourceMap {
	return resourceMap(p.getSchema().DataSourceSchemas)
}

func filter[T any](m []T, keep func(T) bool) []T {
	for i := len(m) - 1; i >= 0; i-- {
		if keep(m[i]) {
			continue
		}

		// If i is not the last element in the slice, put the last element in the
		// slice into i.
		if i < len(m)-1 {
			m[i] = m[len(m)-1]
		}

		// Drop the last element from the slice
		m = m[:i-1]
	}

	return m
}
