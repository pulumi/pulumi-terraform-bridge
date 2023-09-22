package tfbridgetests

import (
	"fmt"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider/sdkv2randomprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"testing"
)

// Benchmark tests based on init of an example large provider

func BenchmarkRandomProviderInit(b *testing.B) {
	for n := 0; n < b.N; n++ {
		testprovider.RandomProvider()
	}
}

func BenchmarkMuxedRandomProviderInit(b *testing.B) {
	for n := 0; n < b.N; n++ {
		testprovider.MuxedRandomProvider()
	}
}

func BenchmarkSizedMuxedRandomProviderInit(b *testing.B) {
	for _, size := range []int{100, 10000} {
		b.Run(fmt.Sprintf("resources_size_%d", size), func(b *testing.B) {
			prov := sdkv2randomprovider.Sized(size)
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				testprovider.MuxedRandomProviderWithSdkProvider(prov)
			}
		})
	}
}

func BenchmarkMuxedRandomProviderMustApplyAutoAliases(b *testing.B) {
	p := testprovider.MuxedRandomProvider()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		p.MustApplyAutoAliases()
	}
}

func BenchmarkSizedMuxedRandomProviderMustApplyAutoAliases(b *testing.B) {
	for _, size := range []int{100, 10000} {
		b.Run(fmt.Sprintf("resources_size_%d", size), func(b *testing.B) {
			prov := sdkv2randomprovider.Sized(size)
			p := testprovider.MuxedRandomProviderWithSdkProvider(prov)
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				p.MustApplyAutoAliases()
			}
		})
	}
}

func BenchmarkLoadMetadata(b *testing.B) {
	for n := 0; n < b.N; n++ {
		tfbridge.NewProviderMetadata(testprovider.BigMetadata)
	}
}
