package shim

import (
	"github.com/opentofu/opentofu/internal/configs/configschema"
	"github.com/opentofu/opentofu/internal/providers"
)

type (
	ProviderSchema = providers.ProviderSchema

	Schema            = providers.Schema
	SchemaBlock       = configschema.Block
	SchemaNestedBlock = configschema.NestedBlock
	SchemaAttribute   = configschema.Attribute
	SchemaObject      = configschema.Object
)
