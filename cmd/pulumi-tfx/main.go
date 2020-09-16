package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/blang/semver"
	"github.com/cheggaaa/pb"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/codegen"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/plugins"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/provider"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/registry"
)

type barCloser struct {
	bar        *pb.ProgressBar
	readCloser io.ReadCloser
}

func (bc *barCloser) Read(dest []byte) (int, error) {
	return bc.readCloser.Read(dest)
}

func (bc *barCloser) Close() error {
	bc.bar.Finish()
	return bc.readCloser.Close()
}

func newProgressBar(closer io.ReadCloser, size int64, message string) io.ReadCloser {
	if size == -1 {
		return closer
	}

	bar := pb.New(int(size))
	bar.Prefix(message + ":")
	bar.SetMaxWidth(80)
	bar.SetUnits(pb.U_BYTES)
	bar.Start()

	return &barCloser{bar: bar, readCloser: bar.NewProxyReader(closer)}
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Args:  cmdutil.NoArgs,
		Short: "List installed Terraform plugins",
		Long: "List installed Terraform plugins.\n" +
			"\n" +
			"This command lists all Terraform plugins that are installed and ready to use with\n" +
			"the TFX resource provider.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			cache, err := plugins.DefaultCache()
			if err != nil {
				return fmt.Errorf("failed to open cache: %w", err)
			}

			plugins, err := cache.ListAllPlugins()
			if err != nil {
				return fmt.Errorf("failed to list plugins: %w", err)
			}
			if len(plugins) == 0 {
				fmt.Println("No plugins are installed.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', 0)
			fmt.Fprintln(w, "REGISTRY\tNAMESPACE\tNAME\tVERSION")
			for _, plugin := range plugins {
				row := strings.Join([]string{
					plugin.RegistryName,
					plugin.Namespace,
					plugin.Name,
					plugin.Version.String(),
				}, "\t")
				fmt.Fprintln(w, row)
			}
			w.Flush()
			return nil
		}),
	}
}

func newInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install [[<registry>/]<namespace>/]<name> [<version range>]",
		Args:  cmdutil.RangeArgs(1, 2),
		Short: "Install a Terraform provider plugin",
		Long: "Install a Terraform provider plugin.\n" +
			"\n" +
			"This command is used to manually install Terraform provider plugins for use with\n" +
			"the TFX resource provider.\n" +
			"\n" +
			"If no version range is specified, the latest version of the given plugin will be\n" +
			"installed. The name 'latest' can be used to refer to the latest version. Ranges\n" +
			"are otherwise specified as in NPM (see https://docs.npmjs.com/misc/semver#ranges\n" +
			"for details).",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			versionRange := func(v semver.Version) bool { return true }
			if len(args) == 2 {
				vr, err := semver.ParseRange(args[1])
				if err != nil {
					return fmt.Errorf("could not parse version range: %w", err)
				}
				versionRange = vr
			}

			meta, err := plugins.ParsePluginReference(args[0])
			if err != nil {
				return err
			}

			registry, err := registry.NewClient(meta.RegistryName)
			if err != nil {
				return err
			}

			cache, err := plugins.DefaultCache()
			if err != nil {
				return fmt.Errorf("failed to open plugin cache: %w", err)
			}

			meta, err = cache.EnsurePlugin(registry, meta.Namespace, meta.Name, versionRange, newProgressBar)
			if err != nil {
				return fmt.Errorf("plugin installation failed: %w", err)
			}
			fmt.Printf("Installed %v.\n", meta)
			return nil
		}),
	}
}

func newQueryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "query [[<registry>/]<namespace>/]<name> [<version range>]",
		Args:  cmdutil.RangeArgs(1, 2),
		Short: "List a Terraform provider plugin's available versions",
		Long: "List a Terraform provider plugin's available versions.\n" +
			"\n" +
			"This command is used to query Terraform provider plugins for use with\n" +
			"the TFX resource provider.\n" +
			"\n" +
			"If no version range is specified, the latest version of the given plugin will be\n" +
			"installed. The name 'latest' can be used to refer to the latest version. Ranges\n" +
			"are otherwise specified as in NPM (see https://docs.npmjs.com/misc/semver#ranges\n" +
			"for details).",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			versionRange := func(v semver.Version) bool { return true }
			if len(args) == 2 {
				vr, err := semver.ParseRange(args[1])
				if err != nil {
					return fmt.Errorf("could not parse version range: %w", err)
				}
				versionRange = vr
			}

			meta, err := plugins.ParsePluginReference(args[0])
			if err != nil {
				return err
			}

			registry, err := registry.NewClient(meta.RegistryName)
			if err != nil {
				return err
			}

			versions, err := registry.ListVersions(meta.Namespace, meta.Name)
			if err != nil {
				return fmt.Errorf("failed to list versions: %w", err)
			}

			var validVersions []semver.Version
			for _, v := range versions {
				sv, err := semver.ParseTolerant(v.Version)
				if err != nil || !versionRange(sv) {
					continue
				}
				if !v.SupportsProtocolVersion(5, 0) || !v.SupportsPlatform(runtime.GOOS, runtime.GOARCH) {
					continue
				}
				validVersions = append(validVersions, sv)
			}

			sort.Slice(validVersions, func(i, j int) bool {
				return validVersions[i].LT(validVersions[j])
			})
			for _, v := range validVersions {
				fmt.Println(v)
			}
			return nil
		}),
	}
}

func addProviderDocs(meta *plugins.PluginMeta, info tfbridge.ProviderInfo) error {
	contract.Assert(meta.Version != nil)

	registry, err := registry.NewV2Client(meta.RegistryName)
	if err != nil {
		return err
	}

	docs, err := registry.GetProviderDocs(meta.Namespace, meta.Name, meta.Version.String())
	if err != nil {
		return err
	}

	for _, doc := range docs {
		switch doc.Category {
		case "resources":
			slug := info.Name + "_" + doc.Slug
			if res, ok := info.Resources[slug]; ok {
				res.Docs = &tfbridge.DocInfo{Markdown: []byte(doc.Content)}
			}
		case "data-sources":
			slug := info.Name + "_" + doc.Slug
			if ds, ok := info.DataSources[slug]; ok {
				ds.Docs = &tfbridge.DocInfo{Markdown: []byte(doc.Content)}
			}
		}
	}

	return nil
}

func fixupPackageJSON(meta *plugins.PluginMeta, path string) error {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %v: %w", path, err)
	}

	var pkg map[string]interface{}
	if err = json.Unmarshal(contents, &pkg); err != nil {
		return fmt.Errorf("failed to read %v: %w", path, err)
	}

	pkg["version"] = "0.0.1"
	pkg["name"] = fmt.Sprintf("@pulumi/tfx/%v", meta)

	contents, err = json.MarshalIndent(pkg, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to write %v: %w", err)
	}
	if err = ioutil.WriteFile(path, contents, 0700); err != nil {
		return fmt.Errorf("failed to write %v: %w", err)
	}
	return nil
}

func newGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate [[<registry>/]<namespace>/]<name> [<version range>] <language> <outdir>",
		Args:  cmdutil.RangeArgs(3, 4),
		Short: "Generate an SDK for a Terraform provider",
		Long: "Generate an SDK for a Terraform provider.\n" +
			"\n" +
			"This command generates a language-specific SDK for a Terraform provider. If the\n" +
			"provider is not installed, this command will install it prior to generating the\n" +
			"requested SDK.\n" +
			"\n" +
			"If no version range is specified, the latest version of the given plugin will be\n" +
			"installed. The name 'latest' can be used to refer to the latest version. Ranges\n" +
			"are otherwise specified as in NPM (see https://docs.npmjs.com/misc/semver#ranges\n" +
			"for details).\n" +
			"\n" +
			"Supported languages are golang, nodejs, python, and dotnet.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			language, outdir := tfgen.Language(args[len(args)-2]), args[len(args)-1]

			versionRange := func(v semver.Version) bool { return true }
			if len(args) == 4 {
				vr, err := semver.ParseRange(args[1])
				if err != nil {
					return fmt.Errorf("could not parse version range: %w", err)
				}
				versionRange = vr
			}

			meta, err := plugins.ParsePluginReference(args[0])
			if err != nil {
				return err
			}

			registry, err := registry.NewClient(meta.RegistryName)
			if err != nil {
				return err
			}

			cache, err := plugins.DefaultCache()
			if err != nil {
				return fmt.Errorf("failed to open plugin cache: %w", err)
			}

			meta, err = cache.EnsurePlugin(registry, meta.Namespace, meta.Name, versionRange, newProgressBar)
			if err != nil {
				return fmt.Errorf("failed to read or install plugin: %w", err)
			}

			cancelContext, cancel := context.WithCancel(context.Background())
			providerInfo, err := provider.StartProvider(cancelContext, *meta)
			if err != nil {
				return fmt.Errorf("failed to launch plugin: %w", err)
			}
			defer cancel()

			// Attempt to add provider docs.
			err = addProviderDocs(meta, providerInfo)
			contract.IgnoreError(err)

			// Stamp over the provider version before generation so that the version of the TFX provider is
			// emitted into the generated code rather than the version of the Terraform plugin.
			providerInfo.Version = "0.0.1"

			outdir, err = filepath.Abs(outdir)
			if err != nil {
				return fmt.Errorf("failed to get absolute path to output directory: %w", err)
			}
			if err = os.MkdirAll(outdir, 0700); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
			root := afero.NewBasePathFs(afero.NewOsFs(), outdir)

			codegenContext, err := codegen.NewContext(cancelContext, cache, codegen.ContextOptions{
				Progress:   newProgressBar,
				PluginMeta: map[string]plugins.PluginMeta{meta.Name: *meta},
			})
			if err != nil {
				return fmt.Errorf("failed to create codegen context: %w", err)
			}

			g, err := tfgen.NewGenerator(tfgen.GeneratorOptions{
				Package:            meta.Name,
				Version:            meta.Version.String(),
				Language:           language,
				ProviderInfo:       providerInfo,
				Root:               root,
				ProviderInfoSource: codegenContext,
				PluginHost:         codegenContext,
				TerraformVersion:   "12",
			})
			if err != nil {
				return fmt.Errorf("failed to initialize code generator: %w", err)
			}
			if err = g.Generate(); err != nil {
				return err
			}

			// Fixup the generated package.json if necessary.
			if language == tfgen.NodeJS {
				if err = fixupPackageJSON(meta, filepath.Join(outdir, "package.json")); err != nil {
					return err
				}
			}

			fmt.Printf("Generated SDK for %v.\n", meta)
			return nil
		}),
	}
}

func main() {
	cmd := &cobra.Command{
		Use:   "pulumi-tfx",
		Args:  cmdutil.NoArgs,
		Short: "Manage Pulumi TFX plugins",
		Long: "Manage Pulumi TFX plugins.\n" +
			"\n" +
			"The Pulumi TFX resource provider uses dynamically loaded Terraform plugins for\n" +
			"supporting arbitrary Terraform resources.",
	}

	cmd.AddCommand(
		newListCommand(),
		newInstallCommand(),
		newQueryCommand(),
		newGenerateCommand())

	if err := cmd.Execute(); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
