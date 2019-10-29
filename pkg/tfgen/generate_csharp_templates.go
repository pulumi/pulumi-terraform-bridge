package tfgen

import (
	"text/template"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
)

// nolint:lll
const csharpUtilitiesTemplateText = `using System;
using System.IO;
using System.Reflection;
using Pulumi;

namespace {{.Namespace}}
{
    static class {{.ClassName}}
    {
        public static string? GetEnv(params string[] names)
        {
            foreach (var n in names)
            {
                var value = Environment.GetEnvironmentVariable(n);
                if (value != null)
                {
                    return value;
                }
            }
            return null;
        }

        static string[] trueValues = { "1", "t", "T", "true", "TRUE", "True" };
        static string[] falseValues = { "0", "f", "F", "false", "FALSE", "False" };
        public static bool? GetEnvBoolean(params string[] names)
        {
            var s = GetEnv(names);
            if (s != null)
            {
                if (Array.IndexOf(trueValues, s) != -1)
                {
                    return true;
                }
                if (Array.IndexOf(falseValues, s) != -1)
                {
                    return false;
                }
            }
            return null;
        }

        public static int? GetEnvInt32(params string[] names) => int.TryParse(GetEnv(names), out int v) ? (int?)v : null;

        public static double? GetEnvDouble(params string[] names) => double.TryParse(GetEnv(names), out double v) ? (double?)v : null;

        public static InvokeOptions WithVersion(this InvokeOptions? options)
        {
            if (options?.Version != null)
            {
                return options;
            }
            return new InvokeOptions
            {
                Parent = options?.Parent,
                Provider = options?.Provider,
                Version = Version,
            };
        }

        private readonly static string version;
        public static string Version => version;

        static Utilities()
        {
            var assembly = typeof(Utilities).GetTypeInfo().Assembly;
            using (var stream = assembly.GetManifestResourceStream("{{.Namespace}}.version.txt"))
            using (var reader = new StreamReader(stream))
            {
                version = reader.ReadToEnd().Trim();
            }
        }
    }
}
`

var csharpUtilitiesTemplate = template.Must(template.New("CSharpUtilities").Parse(csharpUtilitiesTemplateText))

type csharpUtilitiesTemplateContext struct {
	Namespace string
	ClassName string
	Version   string
}

// TODO(pdg): parameterize package name
const csharpProjectFileTemplateText = `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <GeneratePackageOnBuild>true</GeneratePackageOnBuild>
    <Authors>Pulumi Corp.</Authors>
    <Company>Pulumi Corp.</Company>
    <Description>{{.Info.Description}}</Description>
    <PackageLicenseExpression>{{.Info.License}}</PackageLicenseExpression>
    <PackageProjectUrl>{{.Info.Homepage}}</PackageProjectUrl>
    <RepositoryUrl>{{.Info.Repository}}</RepositoryUrl>

    <TargetFramework>netcoreapp3.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>

  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Debug|AnyCPU'">
    <GenerateDocumentationFile>true</GenerateDocumentationFile>
    <NoWarn>1701;1702;1591;8604;8625</NoWarn>
  </PropertyGroup>

  <ItemGroup>
    <EmbeddedResource Include="version.txt" />
  </ItemGroup>

  <ItemGroup>
    {{- range $package, $version := .Info.CSharp.PackageReferences}}
    <PackageReference Include="{{$package}}" Version="{{$version}}" />
    {{- end}}
  </ItemGroup>

</Project>
`

var csharpProjectFileTemplate = template.Must(template.New("CSharpProject").Parse(csharpProjectFileTemplateText))

type csharpProjectFileTemplateContext struct {
	XMLDoc  string
	Info    tfbridge.ProviderInfo
	Version string
}
