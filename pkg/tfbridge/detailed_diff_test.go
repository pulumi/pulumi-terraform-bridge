package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestIsBlock(t *testing.T) {
	tests := []struct {
		name string
		s    shim.Schema
		want bool
	}{
		{
			name: "block",
			s:    shimv2.NewSchema(&schema.Schema{Elem: &schema.Resource{}}),
			want: true,
		},
		{
			name: "schema",
			s:    shimv2.NewSchema(&schema.Schema{Elem: &schema.Schema{}}),
			want: false,
		},
		{
			name: "nil",
			s:    shimv2.NewSchema(&schema.Schema{Elem: nil}),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBlock(tt.s); got != tt.want {
				t.Errorf("isBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakePropDiff(t *testing.T) {
	tests := []struct {
		name  string
		old   resource.PropertyValue
		new   resource.PropertyValue
		oldOk bool
		newOk bool
		want  *pulumirpc.PropertyDiff
	} {
		{
			name: "unchanged non-nil",
			old:  resource.NewStringProperty("same"),
			new:  resource.NewStringProperty("same"),
			oldOk: true,
			newOk: true,
			want: nil,
		},
		{
			name: "unchanged nil",
			old:  resource.NewNullProperty(),
			new:  resource.NewNullProperty(),
			oldOk: true,
			newOk: true,
			want: nil,
		},
		{
			name: "unchanged not present",
			old:  resource.NewNullProperty(),
			new:  resource.NewNullProperty(),
			oldOk: false,
			newOk: false,
			want: nil,
		},
		{
			name: "added",
			old:  resource.NewNullProperty(),
			new:  resource.NewStringProperty("new"),
			oldOk: false,
			newOk: true,
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name: "deleted",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewNullProperty(),
			oldOk: true,
			newOk: false,
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name:  "changed non-nil",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewStringProperty("new"),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name:  "changed from nil",
			old:   resource.NewNullProperty(),
			new:   resource.NewStringProperty("new"),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name:  "changed to nil",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makePropDiff(context.Background(), nil, nil, tt.old, tt.new, tt.oldOk, tt.newOk); got != tt.want {
				t.Errorf("makePropDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}
