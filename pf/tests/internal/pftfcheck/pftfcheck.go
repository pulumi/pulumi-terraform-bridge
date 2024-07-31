package pftfcheck

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
)

type pfServer struct {
	prov *providerbuilder.Provider
}

func (s *pfServer) GRPCProvider() tfprotov6.ProviderServer {
	return providerserver.NewProtocol6(s.prov)()
}

func NewTfDriverPF(t pulcheck.T, dir string, prov *providerbuilder.Provider) *tfcheck.TfDriver {
	providerbuilder.EnsureProviderValid(prov)
	pfServer := &pfServer{prov: prov}
	return tfcheck.NewTFDriverV6(t, dir, prov.TypeName, pfServer)
}
