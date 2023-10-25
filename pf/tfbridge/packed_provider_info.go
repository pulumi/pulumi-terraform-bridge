package tfbridge

import (
	"github.com/json-iterator/go"
	pfmuxer "github.com/pulumi/pulumi-terraform-bridge/pf/internal/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimUtil "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
)

func UnpackProviderInfo(jsonProviderInfo []byte, bareProvider shim.Provider) (*tfbridge.ProviderInfo, error) {
	var marshalled WithProviderAliases
	jsoni := jsoniter.ConfigCompatibleWithStandardLibrary
	err := jsoni.Unmarshal(jsonProviderInfo, &marshalled)
	if err != nil {
		return nil, err
	}
	info := marshalled.Info.Unmarshal()
	info.P = bareProvider

	// Set the maps inside the provider
	for a, t := range marshalled.ResourceAliases {
		info.P.ResourcesMap().AddAlias(a, t)
	}
	for a, t := range marshalled.DatasourceAliases {
		info.P.DataSourcesMap().AddAlias(a, t)
	}

	return info, nil
}

func PackProviderInfo(info *tfbridge.ProviderInfo) ([]byte, error) {
	var resources shimUtil.AliasingResourceMap
	var datasources shimUtil.AliasingResourceMap
	if p, ok := info.P.(*shimUtil.AliasingProvider); ok {
		resources = p.ResourceMap
		datasources = p.DataSourceMap
	} else if p, ok := info.P.(*pfmuxer.ProviderShim); ok {
		resources = p.ResourcesMap().(shimUtil.AliasingResourceMap)
		datasources = p.DataSourcesMap().(shimUtil.AliasingResourceMap)
	} else {
		panic("can't find aliases from this provider")
	}

	resourceAliases := make(map[string]string)
	resources.RangeAliases(func(alias, target string) bool {
		resourceAliases[alias] = target
		return true
	})

	datasourceAliases := make(map[string]string)
	datasources.RangeAliases(func(alias, target string) bool {
		datasourceAliases[alias] = target
		return true
	})

	marshalled := WithProviderAliases{
		Info:              *tfbridge.MarshalProviderInfo(info),
		ResourceAliases:   resourceAliases,
		DatasourceAliases: datasourceAliases,
	}
	jsoni := jsoniter.ConfigCompatibleWithStandardLibrary
	return jsoni.Marshal(marshalled)
}

type WithProviderAliases struct {
	Info              tfbridge.MarshallableProviderInfo `json:"info"`
	ResourceAliases   map[string]string                 `json:"resource_aliases,omitempty"`
	DatasourceAliases map[string]string                 `json:"datasource_aliases,omitempty"`
}
