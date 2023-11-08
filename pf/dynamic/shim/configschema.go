package shim

import (
	"github.com/opentofu/opentofu/internal/configs/configschema"
)

type SchemaStringKind = configschema.StringKind

const (
	SchemaStringPlain    = configschema.StringPlain
	SchemaStringMarkdown = configschema.StringMarkdown
)

type SchemaBlock = configschema.Block
type SchemaAttribute = configschema.Attribute
type SchemaObject = configschema.Object
type NestedBlock = configschema.NestedBlock
type NestingMode = configschema.NestingMode

const (
	NestingSingle  = configschema.NestingSingle
	NestingGroup   = configschema.NestingGroup
	NestingList    = configschema.NestingList
	NestingSet     = configschema.NestingSet
	NestingMap     = configschema.NestingMap
)
