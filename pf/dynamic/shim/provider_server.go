package shim

import (
	"github.com/opentofu/opentofu/internal/grpcwrap"
	"github.com/opentofu/opentofu/internal/providers"
	"github.com/opentofu/opentofu/internal/tfplugin6"
)

func MakeProvider6(mkProv providers.Factory) (tfplugin6.ProviderServer, error) {
	iface, err := mkProv()
	if err != nil {
		return nil, err
	}
	return grpcwrap.Provider6(iface), nil
}
