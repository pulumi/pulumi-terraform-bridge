package adapter

import (
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestConvert(t *testing.T) {
	data, err := Convert("testdata/bucket_state.json", "")
	if err != nil {
		t.Fatalf("failed to convert Terraform state: %v", err)
	}

	autogold.ExpectFile(t, data)
}
