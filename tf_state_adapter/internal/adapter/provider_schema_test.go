package adapter

import (
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestGetProviderSchema(t *testing.T) {
	info, err := getProviderSchema("aws")
	if err != nil {
		t.Fatalf("failed to get provider schema: %v", err)
	}
	autogold.ExpectFile(t, info.Resources["aws_s3_bucket"].Tok)
}
