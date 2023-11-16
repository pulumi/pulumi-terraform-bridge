package tfbridge

import (
	"testing"

	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataInfo(t *testing.T) {
	data, err := md.New(nil)
	require.NoError(t, err)

	err = md.Set(data, "hi", []string{"hello", "world"})
	require.NoError(t, err)
	err = md.Set(data, "auto-aliasing", []string{"1", "2"})
	require.NoError(t, err)
	err = md.Set(data, "mux", []string{"a", "b"})
	require.NoError(t, err)

	marshalled := data.Marshal()
	require.Equal(t, `{"auto-aliasing":["1","2"],"hi":["hello","world"],"mux":["a","b"]}`, string(marshalled))

	info := NewProviderMetadata(marshalled)
	assert.Equal(t, "bridge-metadata.json", info.Path)
	marshalledInfo := (*md.Data)(info.Data).Marshal()
	assert.Equal(t, `{"auto-aliasing":["1","2"],"hi":["hello","world"],"mux":["a","b"]}`, string(marshalledInfo))

	runtimeMetadata := info.ExtractRuntimeMetadata()
	assert.Equal(t, "runtime-bridge-metadata.json", runtimeMetadata.Path)
	runtimeMarshalled := (*md.Data)(runtimeMetadata.Data).Marshal()
	assert.Equal(t, `{"auto-aliasing":["1","2"],"mux":["a","b"]}`, string(runtimeMarshalled))
}
