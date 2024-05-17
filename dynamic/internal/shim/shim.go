package shim

import (
	"github.com/opentofu/opentofu/internal/configs/configschema"
	"github.com/opentofu/opentofu/internal/providers"
	"github.com/opentofu/opentofu/internal/tfdiags"
	"github.com/opentofu/opentofu/internal/tfplugin6"
)

type (
	Diagnostics = tfdiags.Diagnostics

	ProviderSchema = providers.ProviderSchema

	Schema            = providers.Schema
	SchemaBlock       = configschema.Block
	SchemaNestedBlock = configschema.NestedBlock
	SchemaAttribute   = configschema.Attribute
	SchemaObject      = configschema.Object

	ValidateProviderConfigRequest  = providers.ValidateProviderConfigRequest
	ValidateProviderConfigResponse = providers.ValidateProviderConfigResponse

	ValidateResourceConfigRequest  = providers.ValidateResourceConfigRequest
	ValidateResourceConfigResponse = providers.ValidateResourceConfigResponse

	ValidateDataResourceConfigRequest  = providers.ValidateDataResourceConfigRequest
	ValidateDataResourceConfigResponse = providers.ValidateDataResourceConfigResponse

	ConfigureProviderRequest  = providers.ConfigureProviderRequest
	ConfigureProviderResponse = providers.ConfigureProviderResponse
)

var (
	DiagError = tfdiags.Error
)

// RawGRPC
type (
	GetMetadata_Request = tfplugin6.GetMetadata_Request
)
