package fallbackstrat

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

func tokenStrategyWithFallback(
	strategy tokens.Strategy,
	fallback tokens.Strategy,
) tokens.Strategy {
	resourceFallback := func(tfToken string, elem *info.Resource) error {
		if err := strategy.Resource(tfToken, elem); err != nil {
			return fallback.Resource(tfToken, elem)
		}
		return nil
	}
	dataSourceFallback := func(tfToken string, elem *info.DataSource) error {
		if err := strategy.DataSource(tfToken, elem); err != nil {
			return fallback.DataSource(tfToken, elem)
		}
		return nil
	}
	return tokens.Strategy{
		Resource:   resourceFallback,
		DataSource: dataSourceFallback,
	}
}

func KnownModulesWithInferredFallback(
	p *info.Provider, tfPackagePrefix, defaultModule string, modules []string, finalize tokens.Make,
) (tokens.Strategy, error) {
	opts := &tokens.InferredModulesOpts{
		TfPkgPrefix:          tfPackagePrefix,
		MinimumModuleSize:    2,
		MimimumSubmoduleSize: 2,
	}
	inferred, err := tokens.InferredModules(p, finalize, opts)
	if err != nil {
		return tokens.Strategy{}, err
	}
	return tokenStrategyWithFallback(
		tokens.KnownModules(tfPackagePrefix, defaultModule, modules, finalize),
		inferred,
	), nil
}

func MappedModulesWithInferredFallback(
	p *info.Provider, tfPackagePrefix, defaultModule string, modules map[string]string, finalize tokens.Make,
) (tokens.Strategy, error) {
	inferred, err := tokens.InferredModules(p, finalize, &tokens.InferredModulesOpts{
		TfPkgPrefix:          tfPackagePrefix,
		MinimumModuleSize:    2,
		MimimumSubmoduleSize: 2,
	})
	if err != nil {
		return tokens.Strategy{}, err
	}
	return tokenStrategyWithFallback(
		tokens.MappedModules(tfPackagePrefix, defaultModule, modules, finalize),
		inferred,
	), nil
}
