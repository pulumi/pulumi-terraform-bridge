package history

import "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

type TokenHistory[T ~string] struct {
	Current T          `json:"current"`        // the current Pulumi token for the resource
	Past    []Alias[T] `json:"past,omitempty"` // Previous tokens

	MajorVersion int                      `json:"majorVersion,omitempty"`
	Fields       map[string]*FieldHistory `json:"fields,omitempty"`
}

type Alias[T ~string] struct {
	Name         T    `json:"name"`         // The previous token.
	InCodegen    bool `json:"inCodegen"`    // If the alias is a fully generated resource, or just a schema alias.
	MajorVersion int  `json:"majorVersion"` // The provider's major version when Name was introduced.
}

type AliasHistory struct {
	Resources   map[string]*TokenHistory[tokens.Type]         `json:"resources,omitempty"`
	DataSources map[string]*TokenHistory[tokens.ModuleMember] `json:"datasources,omitempty"`
}

type FieldHistory struct {
	MaxItemsOne *bool `json:"maxItemsOne,omitempty"`

	Fields map[string]*FieldHistory `json:"fields,omitempty"`
	Elem   *FieldHistory            `json:"elem,omitempty"`
}
