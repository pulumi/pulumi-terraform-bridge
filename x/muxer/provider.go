package muxer

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Provider defines an interface which must be implemented by providers
// that shall be used as mixins of a wrapped Terraform provider
type Provider interface {
	GetSpec() (schema.PackageSpec, error)
	GetInstance(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error)
}
