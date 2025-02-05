package fallbackstrat_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens/fallbackstrat"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestTokensMappedModulesWithInferredFallback(t *testing.T) {
	t.Parallel()
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"cs101_fizz_buzz_one_five": nil,
				"cs101_fizz_three":         nil,
				"cs101_fizz_three_six":     nil,
				"cs101_buzz_five":          nil,
				"cs101_buzz_ten":           nil,
			},
		}).Shim(),
	}
	strategy, err := fallbackstrat.MappedModulesWithInferredFallback(
		&info,
		"cs101_", "", map[string]string{
			"fizz_":      "fIzZ",
			"fizz_buzz_": "fizZBuzz",
		},
		func(module, name string) (string, error) {
			return fmt.Sprintf("cs101:%s:%s", module, name), nil
		},
	)
	require.NoError(t, err)

	err = info.ComputeTokens(tfbridge.Strategy{
		Resource: strategy.Resource,
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizZBuzz:OneFive"},
		"cs101_fizz_three":         {Tok: "cs101:fIzZ:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fIzZ:ThreeSix"},
		// inferred
		"cs101_buzz_five": {Tok: "cs101:buzz:Five"},
		"cs101_buzz_ten":  {Tok: "cs101:buzz:Ten"},
	}, info.Resources)
}

func TestTokensKnownModulesWithInferredFallback(t *testing.T) {
	t.Parallel()
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"cs101_fizz_buzz_one_five": nil,
				"cs101_fizz_three":         nil,
				"cs101_fizz_three_six":     nil,
				"cs101_buzz_five":          nil,
				"cs101_buzz_ten":           nil,
			},
		}).Shim(),
	}

	strategy, err := fallbackstrat.KnownModulesWithInferredFallback(&info,
		"cs101_", "", []string{
			"fizz_", "fizz_buzz_",
		}, func(module, name string) (string, error) {
			return fmt.Sprintf("cs101:%s:%s", module, name), nil
		})
	require.NoError(t, err)

	err = info.ComputeTokens(tfbridge.Strategy{
		Resource: strategy.Resource,
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:OneFive"},
		"cs101_fizz_three":         {Tok: "cs101:fizz:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fizz:ThreeSix"},
		// inferred
		"cs101_buzz_five": {Tok: "cs101:buzz:Five"},
		"cs101_buzz_ten":  {Tok: "cs101:buzz:Ten"},
	}, info.Resources)
}
