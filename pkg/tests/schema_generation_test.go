package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

func marshalPackageSpec(spec pschema.PackageSpec) (string, error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(spec)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestRequiredInputWithDefault(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - tests cases need to be made robust to newline handling")
	}

	p := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"testprovider_res": {
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
						DefaultFunc: func() (interface{}, error) {
							return "default", nil
						},
					},
					"req": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
	}

	provider := pulcheck.BridgedProvider(t, "testprovider", p)

	schema, err := tfgen.GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)
	require.Empty(t, schema.Resources["testprovider:index:Res"].RequiredInputs)
	spec, err := marshalPackageSpec(schema)
	require.NoError(t, err)
	autogold.ExpectFile(t, autogold.Raw(spec))

	resourceSchema := schema.Resources["testprovider:index/res:Res"]
	require.NotContains(t, resourceSchema.RequiredInputs, "name")
}

func TestRequiredInputWithDefaultFlagDisabled(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - tests cases need to be made robust to newline handling")
	}

	p := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"testprovider_res": {
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
						DefaultFunc: func() (interface{}, error) {
							return "default", nil
						},
					},
					"req": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
	}

	provider := pulcheck.BridgedProvider(t, "testprovider", p)
	provider.DisableRequiredWithDefaultTurningOptional = true

	schema, err := tfgen.GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)
	require.Empty(t, schema.Resources["testprovider:index:Res"].RequiredInputs)
	spec, err := marshalPackageSpec(schema)
	require.NoError(t, err)
	autogold.ExpectFile(t, autogold.Raw(spec))

	resourceSchema := schema.Resources["testprovider:index/res:Res"]
	require.Contains(t, resourceSchema.RequiredInputs, "name")
}

func skipOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - tests cases need to be made robust to newline handling")
	}
}

func Test_Generate(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	p := pulcheck.BridgedProvider(t, "prov", &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {Schema: map[string]*schema.Schema{
				"test": {Type: schema.TypeString, Optional: true},
			}},
		},
	})

	sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	outDir := t.TempDir()

	// Create the output directory.
	var root afero.Fs
	if outDir != "" {
		absOutDir, err := filepath.Abs(outDir)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(absOutDir, 0o700))
		root = afero.NewBasePathFs(afero.NewOsFs(), absOutDir)
	}

	gen, err := tfgen.NewGenerator(tfgen.GeneratorOptions{
		Package:       "prov",
		Version:       "0.0.1",
		ProviderInfo:  p,
		Root:          root,
		Language:      tfgen.NodeJS,
		XInMemoryDocs: true,
		SkipDocs:      true,
		SkipExamples:  true,
		Sink:          sink,
		Debug:         true,
	})
	require.NoError(t, err)

	_, err = gen.Generate()
	require.NoError(t, err)

	err = afero.Walk(root, ".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, err := afero.ReadFile(root, path)
		if err != nil {
			return err
		}
		t.Logf("file: %s", path)
		t.Logf("content: %s", string(content))
		return nil
	})
	require.NoError(t, err)
}

func Test_GenerateWithOverlay(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	p := pulcheck.BridgedProvider(t, "prov", &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {Schema: map[string]*schema.Schema{
				"test": {Type: schema.TypeString, Optional: true},
			}},
			"prov_test_mod": {Schema: map[string]*schema.Schema{
				"test": {Type: schema.TypeString, Optional: true},
			}},
		},
	})

	moduleName := "mod"
	// overwrite the token to ensure the module is generated
	p.Resources["prov_test_mod"].Tok = tokens.Type("prov:mod/Mod:Test")

	sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	language := tfgen.NodeJS

	overlayFileName := "hello.ts"
	overlayFileContent := []byte(`
	export const hello = "world";
	`)

	overlayModFileName := "helloMod.ts"
	overlayModFileContent := []byte(`
	export const helloMod = "worldMod";
	`)

	if p.JavaScript == nil {
		p.JavaScript = &info.JavaScript{}
	}
	p.JavaScript.Overlay = &info.Overlay{
		DestFiles: []string{
			overlayFileName,
		},
		Modules: map[string]*info.Overlay{
			moduleName: {
				DestFiles: []string{
					overlayModFileName,
				},
			},
		},
	}

	// Create the output directory.
	root := afero.NewMemMapFs()

	err := afero.WriteFile(root, overlayFileName, overlayFileContent, 0o600)
	require.NoError(t, err)

	err = afero.WriteFile(root, filepath.Join(moduleName, overlayModFileName), overlayModFileContent, 0o600)
	require.NoError(t, err)

	gen, err := tfgen.NewGenerator(tfgen.GeneratorOptions{
		Package:       "prov",
		Version:       "0.0.1",
		ProviderInfo:  p,
		Root:          root,
		Language:      language,
		XInMemoryDocs: true,
		SkipDocs:      true,
		SkipExamples:  true,
		Sink:          sink,
		Debug:         true,
	})
	require.NoError(t, err)

	_, err = gen.Generate()
	require.NoError(t, err)

	content, err := afero.ReadFile(root, filepath.Join(string(language), overlayFileName))
	require.NoError(t, err)
	require.Equal(t, overlayFileContent, content)

	content, err = afero.ReadFile(root, filepath.Join(string(language), moduleName, overlayModFileName))
	require.NoError(t, err)
	require.Equal(t, overlayModFileContent, content)

	err = afero.Walk(root, ".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, err := afero.ReadFile(root, path)
		if err != nil {
			return err
		}
		t.Logf("file: %s", path)
		t.Logf("content: %s", string(content))
		return nil
	})
	require.NoError(t, err)
}
