package tfbridgetests

import (
	"context"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testprovider"
)

func TestSchemaGen(t *testing.T) {
	ctx := context.Background()
	sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	})
	provider := testprovider.RandomProvider()
	shimProvider := schemashim.ShimSchemaOnlyProvider(ctx, provider.P())
	info := tfbridge.ProviderInfo{
		P: shimProvider,
	}

	_, err := tfgen.GenerateSchema(info, sink)
	require.NoError(t, err)

}
