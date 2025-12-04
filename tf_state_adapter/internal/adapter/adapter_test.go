package adapter

import (
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestConvertSimple(t *testing.T) {
	data, err := ConvertState("testdata/bucket_state.json", "")
	if err != nil {
		t.Fatalf("failed to convert Terraform state: %v", err)
	}

	autogold.ExpectFile(t, data)
}

func TestConvertInvolved(t *testing.T) {
	data, err := ConvertState("testdata/tofu_state.json", "")
	if err != nil {
		t.Fatalf("failed to convert Terraform state: %v", err)
	}

	autogold.ExpectFile(t, data)
}

func TestConvertResourceState(t *testing.T) {
	data, err := ConvertResourceState("testdata/bucket_state.json", "aws_s3_bucket.example", "")
	if err != nil {
		t.Fatalf("failed to convert Terraform resource state: %v", err)
	}

	autogold.ExpectFile(t, data)
}
