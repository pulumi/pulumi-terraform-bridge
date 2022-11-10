package tfbridgetests

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testprovider"
)

func TestSchemaGen(t *testing.T) {
	ctx := context.Background()
	sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	})
	info := schemashim.ShimSchemaOnlyProviderInfo(ctx, testprovider.RandomProvider())
	schema, err := tfgen.GenerateSchema(info, sink)
	require.NoError(t, err)

	bytes, err := json.MarshalIndent(schema, "", "  ")
	require.NoError(t, err)
	t.Logf("SCHEMA:\n%v", string(bytes))

	if f := os.Getenv("PULUMI_SAVE_RANDOM_SCHEMA"); f != "" {
		err := os.WriteFile(f, bytes, 0700)
		require.NoError(t, err)
	}
}
