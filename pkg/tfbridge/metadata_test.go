package tfbridge

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

func TestMetadataInfo(t *testing.T) {
	data, err := md.New(nil)
	require.NoError(t, err)

	err = md.Set(data, "hi", []string{"hello", "world"})
	require.NoError(t, err)
	err = md.Set(data, aliasMetadataKey, []string{"1", "2"})
	require.NoError(t, err)
	err = md.Set(data, autoSettingsKey, []string{"..."})
	require.NoError(t, err)
	err = md.Set(data, "mux", []string{"a", "b"})
	require.NoError(t, err)

	marshalled := data.MarshalIndent()
	autogold.Expect(`{
    "auto-aliasing": [
        "1",
        "2"
    ],
    "auto-settings": [
        "..."
    ],
    "hi": [
        "hello",
        "world"
    ],
    "mux": [
        "a",
        "b"
    ]
}`).Equal(t, string(marshalled))

	info := NewProviderMetadata(marshalled)
	assert.Equal(t, "bridge-metadata.json", info.Path)
	marshalledInfo := (*md.Data)(info.Data).MarshalIndent()
	autogold.Expect(`{
    "auto-aliasing": [
        "1",
        "2"
    ],
    "auto-settings": [
        "..."
    ],
    "hi": [
        "hello",
        "world"
    ],
    "mux": [
        "a",
        "b"
    ]
}`).Equal(t, string(marshalledInfo))

	runtimeMetadata := info.ExtractRuntimeMetadata()
	assert.Equal(t, "runtime-bridge-metadata.json", runtimeMetadata.Path)
	runtimeMarshalled := (*md.Data)(runtimeMetadata.Data).MarshalIndent()
	autogold.Expect(`{
    "auto-settings": [
        "..."
    ],
    "mux": [
        "a",
        "b"
    ]
}`).Equal(t, string(runtimeMarshalled))
}
