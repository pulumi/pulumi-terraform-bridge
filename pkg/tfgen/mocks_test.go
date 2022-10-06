package tfgen

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type mockResource struct {
	schema shim.SchemaMap
	docs   tfbridge.DocInfo
	token  tokens.Token
}

func (r *mockResource) GetFields() map[string]*tfbridge.SchemaInfo {
	//TODO implement me
	panic("implement me")
}

func (r *mockResource) ReplaceExamplesSection() bool {
	//TODO implement me
	panic("implement me")
}

func (r *mockResource) GetDocs() *tfbridge.DocInfo {
	return &r.docs
}

func (r *mockResource) GetTok() tokens.Token {
	return r.token
}

func (r mockResource) Schema() shim.SchemaMap {
	return r.schema
}

func (r mockResource) SchemaVersion() int {
	//TODO implement me
	panic("implement me")
}

func (r mockResource) Importer() shim.ImportFunc {
	//TODO implement me
	panic("implement me")
}

func (r mockResource) DeprecationMessage() string {
	//TODO implement me
	panic("implement me")
}

func (r mockResource) Timeouts() *shim.ResourceTimeout {
	//TODO implement me
	panic("implement me")
}

func (r mockResource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	//TODO implement me
	panic("implement me")
}

func (r mockResource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	//TODO implement me
	panic("implement me")
}
