package tokens

import "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"

func tokenStrategyWithFallback(
	strategy Strategy,
	fallback Strategy,
) Strategy {
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
	return Strategy{
		Resource:   resourceFallback,
		DataSource: dataSourceFallback,
	}
}
