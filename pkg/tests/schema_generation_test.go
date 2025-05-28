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

	_, err = afero.ReadFile(root, "test.ts")
	require.NoError(t, err)
	_, err = afero.ReadFile(root, "package.json")
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

	overlayFileNameTs := "hello.ts"
	overlayFileContentTs := []byte(`
	export const hello = "world";
	`)

	overlayModFileNameTs := "helloMod.ts"
	overlayModFileContentTs := []byte(`
	export const helloMod = "worldMod";
	`)

	p.JavaScript = &info.JavaScript{
		Overlay: &info.Overlay{
			DestFiles: []string{
				overlayFileNameTs,
			},
			Modules: map[string]*info.Overlay{
				moduleName: {
					DestFiles: []string{
						overlayModFileNameTs,
					},
				},
			},
		},
	}

	overlayFileNamePy := "hello.py"
	overlayFileContentPy := []byte(`
	hello = "world"
	`)

	overlayModFileNamePy := "helloMod.py"
	overlayModFileContentPy := []byte(`
	helloMod = "worldMod"
	`)
	p.Python = &info.Python{
		Overlay: &info.Overlay{
			DestFiles: []string{
				overlayFileNamePy,
			},
			Modules: map[string]*info.Overlay{
				moduleName: {
					DestFiles: []string{
						overlayModFileNamePy,
					},
				},
			},
		},
	}

	overlayFileNameGo := "hello.go"
	overlayFileContentGo := []byte(`
	package main

	func hello() {
		fmt.Println("Hello, World!")
	}`)

	overlayModFileNameGo := "helloMod.go"
	overlayModFileContentGo := []byte(`
	package main

	func helloMod() {
		fmt.Println("Hello, World!")
	}`)

	p.Golang = &info.Golang{
		Overlay: &info.Overlay{
			DestFiles: []string{
				overlayFileNameGo,
			},
			Modules: map[string]*info.Overlay{
				moduleName: {
					DestFiles: []string{
						overlayModFileNameGo,
					},
				},
			},
		},
	}

	overlayFileNameCs := "hello.cs"
	overlayFileContentCs := []byte(`
	public class Hello {
		public static void Main(string[] args) {
			Console.WriteLine("Hello, World!");
		}
	}`)

	overlayModFileNameCs := "helloMod.cs"
	overlayModFileContentCs := []byte(`
	public class HelloMod {
		public static void Main(string[] args) {
			Console.WriteLine("Hello, World!");
		}
	}`)

	p.CSharp = &info.CSharp{
		Overlay: &info.Overlay{
			DestFiles: []string{
				overlayFileNameCs,
			},
			Modules: map[string]*info.Overlay{
				moduleName: {
					DestFiles: []string{
						overlayModFileNameCs,
					},
				},
			},
		},
	}

	testCases := []struct {
		name                  string
		language              tfgen.Language
		overlayFileName       string
		overlayModFileName    string
		overlayFileContent    []byte
		overlayModFileContent []byte
	}{
		{
			name: "NodeJS", language: tfgen.NodeJS,
			overlayFileName:       overlayFileNameTs,
			overlayModFileName:    overlayModFileNameTs,
			overlayFileContent:    overlayFileContentTs,
			overlayModFileContent: overlayModFileContentTs,
		},
		{
			name: "Python", language: tfgen.Python,
			overlayFileName:       overlayFileNamePy,
			overlayModFileName:    overlayModFileNamePy,
			overlayFileContent:    overlayFileContentPy,
			overlayModFileContent: overlayModFileContentPy,
		},
		{
			name: "Golang", language: tfgen.Golang,
			overlayFileName:       overlayFileNameGo,
			overlayModFileName:    overlayModFileNameGo,
			overlayFileContent:    overlayFileContentGo,
			overlayModFileContent: overlayModFileContentGo,
		},
		{
			name: "CSharp", language: tfgen.CSharp,
			overlayFileName:       overlayFileNameCs,
			overlayModFileName:    overlayModFileNameCs,
			overlayFileContent:    overlayFileContentCs,
			overlayModFileContent: overlayModFileContentCs,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			osFs := afero.NewOsFs()
			tempDir := t.TempDir()
			root := afero.NewBasePathFs(osFs, tempDir)

			err := afero.WriteFile(root, tc.overlayFileName, tc.overlayFileContent, 0o600)
			require.NoError(t, err)

			err = root.MkdirAll(moduleName, 0o700)
			require.NoError(t, err)

			err = afero.WriteFile(root, filepath.Join(moduleName, tc.overlayModFileName), tc.overlayModFileContent, 0o600)
			require.NoError(t, err)

			gen, err := tfgen.NewGenerator(tfgen.GeneratorOptions{
				Package:       "prov",
				Version:       "0.0.1",
				ProviderInfo:  p,
				Root:          root,
				Language:      tc.language,
				XInMemoryDocs: true,
				SkipDocs:      true,
				SkipExamples:  true,
				Sink:          sink,
				Debug:         true,
			})
			require.NoError(t, err)

			_, err = gen.Generate()
			require.NoError(t, err)

			content, err := afero.ReadFile(root, tc.overlayFileName)
			require.NoError(t, err)
			require.Equal(t, tc.overlayFileContent, content)

			content, err = afero.ReadFile(root, filepath.Join(moduleName, tc.overlayModFileName))
			require.NoError(t, err)
			require.Equal(t, tc.overlayModFileContent, content)

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
		})
	}
}
