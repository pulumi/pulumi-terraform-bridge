package tfgen

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func plainDocsParser(docFile *DocFile, g *Generator) ([]byte, error) {
	// Apply pre-code translation edit rules. This applies all default edit rules and provider-supplied edit rules in
	// the default pre-code translation phase.
	contentBytes, err := g.editRules.apply(docFile.FileName, docFile.Content, info.PreCodeTranslation)
	if err != nil {
		return nil, err
	}

	// Get file content without front matter
	content := trimFrontMatter(contentBytes)

	providerDisplayName := getProviderDisplayName(g)

	// Add pulumi-specific front matter
	// Generate pulumi-specific front matter
	frontMatter := writeFrontMatter(providerDisplayName)

	// Remove the title. A title gets populated from Hugo frontmatter; we do not want two.
	content, err = removeTitle(content)
	if err != nil {
		return nil, err
	}

	// Strip the tfplugindocs generation HTML comment
	content = stripSchemaGeneratedByTFPluginDocs(content)

	// Generate pulumi-specific installation instructions
	installationInstructions := writeInstallationInstructions(
		g.info.Golang.ImportBasePath,
		providerDisplayName,
		g.pkg.Name().String(),
		cSharpPackageName(g.info, g.pkg.Name().String()),
		g.info.GitHubOrg,
		g.info.Repository,
	)

	// Determine if we should write an overview header.
	overviewHeader := getOverviewHeader(content)

	// Translate code blocks to Pulumi
	contentStr, err := translateCodeBlocks(string(content), g)
	if err != nil {
		return nil, err
	}

	// If the code translation resulted in an empty examples section, remove it
	content, err = removeEmptySection("Example Usage", []byte(contentStr))
	if err != nil {
		return nil, err
	}

	// Apply post-code translation edit rules. This applies all default edit rules and provider-supplied edit rules in
	// the post-code translation phase.
	contentBytes, err = g.editRules.apply(docFile.FileName, content, info.PostCodeTranslation)
	if err != nil {
		return nil, err
	}
	// Reformat field names.
	contentStr = reformatText(infoContext{
		language: RegistryDocs,
		pkg:      g.pkg,
		info:     g.info,
	}, string(contentBytes), nil)

	// Add instructions to top of file
	// Instructions need to be added _after_ the editRules are called,
	// because if "hashicorp" or "terraform" show up in the dynamic provider source, we want that to remain.
	contentStr = frontMatter + installationInstructions + overviewHeader + contentStr

	return []byte(contentStr), nil
}

func writeFrontMatter(providerDisplayName string) string {
	return fmt.Sprintf(delimiter+
		"# *** WARNING: This file was auto-generated. "+
		"Do not edit by hand unless you're certain you know what you are doing! ***\n"+
		"title: %[1]s Provider\n"+
		"meta_desc: Provides an overview on how to configure the Pulumi %[1]s provider.\n"+
		"layout: package\n"+
		delimiter+
		"\n",
		providerDisplayName)
}

// writeInstallationInstructions renders the following for a pulumi-maintained provider:
//
//	Installation
//	The Foo provider is available as a package in all Pulumi languages:
//
//	JavaScript/TypeScript: @pulumi/foo
//	Python: pulumi-foo
//	Go: github.com/pulumi/pulumi-foo/sdk/v3/go/foo
//	.NET: Pulumi.foo
//	Java: com.pulumi/foo
//
// A dynamically bridged provider receives the following instead:
//
//	## Generate Provider
//
//	The Foo provider must be installed as a Local Package by following the instructions for Any Terraform Provider:
//	(link)
//	```bash
//	pulumi package add terraform-provider org/foo
//	```
func writeInstallationInstructions(goImportBasePath, displayName, pkgName, cSharpName, ghOrg, sourceRepo string) string {
	installInstructions := fmt.Sprintf(
		"## Installation\n\n"+
			"The %[1]s provider is available as a package in all Pulumi languages:\n\n"+
			"* JavaScript/TypeScript: [`@pulumi/%[2]s`](https://www.npmjs.com/package/@pulumi/%[2]s)\n"+
			"* Python: [`pulumi-%[2]s`](https://pypi.org/project/pulumi-%[2]s/)\n"+
			"* Go: [`%[4]s`](https://github.com/pulumi/pulumi-%[2]s)\n"+
			"* .NET: [`Pulumi.%[3]s`](https://www.nuget.org/packages/Pulumi.%[3]s)\n"+
			"* Java: [`com.pulumi/%[2]s`](https://central.sonatype.com/artifact/com.pulumi/%[2]s)\n\n",
		displayName,
		pkgName,
		cSharpName,
		goImportBasePath,
	)

	generateInstructions := fmt.Sprintf("## Generate Provider\n\n"+
		"The %[1]s provider must be installed as a Local Package by following the "+
		"[instructions for Any Terraform Provider]"+
		"(https://www.pulumi.com/registry/packages/terraform-provider/):\n\n"+
		"```bash\n"+
		"pulumi package add terraform-provider %[2]s/%[3]s\n"+
		"```\n",
		displayName,
		ghOrg,
		pkgName,
	)

	deprecatedNote := fmt.Sprintf("~> **NOTE:** This provider was previously published as @pulumi/%[1]s.\n"+
		"However, that package is no longer being updated."+
		"Going forward, it is available as a [Local Package](https://www.pulumi.com/blog/any-terraform-provider/)"+
		" instead.\n"+
		"Please see the [provider's repository](https://github.com/pulumi/pulumi-%[1]s) for details.\n\n", pkgName)

	if strings.Contains(sourceRepo, "pulumi") {
		return installInstructions
	}
	for _, provider := range getDeprecatedProviderNames() {
		if provider == pkgName {
			// append the deprecation note
			generateInstructions = generateInstructions + deprecatedNote
		}
	}
	return generateInstructions
}

// cSharpPackageName returns the .NET name for a package: its capitalization from CSharp.Namespaces when the provider
// declares one (e.g. "azuread" -> "AzureAD"), otherwise the title-cased package name.
func cSharpPackageName(providerInfo tfbridge.ProviderInfo, pkgName string) string {
	if providerInfo.CSharp != nil {
		if name, ok := providerInfo.CSharp.Namespaces[pkgName]; ok {
			return name
		}
	}
	return cases.Title(language.English).String(pkgName)
}

// TODO: remove this note after 90 days: https://github.com/pulumi/pulumi-terraform-bridge/issues/3304
func getDeprecatedProviderNames() []string {
	providerNames := []string{
		"wavefront",
	}
	return providerNames
}

func getOverviewHeader(content []byte) string {
	const overviewHeader = "## Overview\n\n"
	// If content starts with an H2, or is otherwise empty then we don't want to add this header
	content = bytes.TrimSpace(content)
	if bytes.HasPrefix(content, []byte("## ")) || len(content) == 0 {
		return ""
	}
	return overviewHeader
}

var tfplugindocsComment = regexp.MustCompile("<!-- schema generated by tfplugindocs -->")

func stripSchemaGeneratedByTFPluginDocs(content []byte) []byte {
	content = tfplugindocsComment.ReplaceAll(content, nil)
	return content
}

func translateCodeBlocks(contentStr string, g *Generator) (string, error) {
	var returnContent string
	// Extract code blocks
	const codeFence string = "```"
	codeBlocks := findCodeBlocks([]byte(contentStr))
	if len(codeBlocks) == 0 {
		return contentStr, nil
	}
	startIndex := 0
	for i, block := range codeBlocks {
		// Write the content up to the start of the code block
		returnContent = returnContent + contentStr[startIndex:block.start]
		nextNewLine := strings.Index(contentStr[block.start:block.end], "\n")
		if nextNewLine == -1 {
			// Write the inline block
			returnContent = returnContent + contentStr[block.start:block.end] + codeFence + "\n"
			continue
		}

		// Only convert code blocks that we have reasonable suspicion of actually being Terraform.
		if code := block.code([]byte(contentStr)); isHCL(block.language, code) {
			exampleContent, err := convertExample(g, code, i)
			if err != nil {
				return "", err
			}
			returnContent += exampleContent
			startIndex = block.end + len(codeFence)
		} else {
			// Write any code block as-is.
			returnContent = returnContent + contentStr[block.start:block.end+len(codeFence)]
			startIndex = block.end + len(codeFence)
		}
	}
	// Write any remainder.
	returnContent = returnContent + contentStr[codeBlocks[len(codeBlocks)-1].end+len(codeFence):]
	return returnContent, nil
}

// configLeaf is one `pulumi config set` command.
type configLeaf struct {
	// namespace is the config namespace, such as "aws".
	namespace string
	// path is the config key, followed by the path to the value within it when the value is nested.
	path   resource.PropertyPath
	value  string
	secret bool
}

func newConfigLeaf(namespace string, path resource.PropertyPath, value string, secret bool) configLeaf {
	return configLeaf{namespace: namespace, path: path, value: value, secret: secret}
}

// nested reports whether the leaf sets a value inside a config key, so it needs `--path`.
func (l configLeaf) nested() bool { return len(l.path) > 1 }

// renderConfigSetCommands renders config leaves as a bash block of `pulumi config set` commands. Returns "" if there
// are none.
func renderConfigSetCommands(leaves []configLeaf) string {
	if len(leaves) == 0 {
		return ""
	}
	var w strings.Builder
	w.WriteString("```bash\n")
	for _, leaf := range leaves {
		w.WriteString("pulumi config set ")
		if leaf.secret {
			w.WriteString("--secret ")
		}
		if leaf.nested() {
			// The path is formatted the way `pulumi config set --path` parses it. Always quote paths: list indexes
			// contain shell glob characters.
			path := leaf.namespace + ":" + leaf.path.String()
			w.WriteString("--path '" + strings.ReplaceAll(path, "'", `'\''`) + "'")
		} else {
			w.WriteString(shellQuote(fmt.Sprintf("%s:%v", leaf.namespace, leaf.path[0])))
		}
		w.WriteString(" " + configValueArg(leaf.value) + "\n")
	}
	w.WriteString("```\n")
	return w.String()
}

// exampleConfigLeaves returns the config set by the provider blocks in an HCL example.
//
// This provider's blocks are read from the HCL against its schema. The converter's Pulumi.yaml can't be used for
// them: it drops nested blocks in provider config and keeps nested keys in Terraform casing.
//
// Config for any other provider still comes from the converter's Pulumi.yaml, because only the converter knows that
// provider's Pulumi package name and schema. Install docs rarely configure another provider, but it does happen. For
// example, databricks configures azurerm alongside itself:
// https://registry.terraform.io/providers/databricks/databricks/latest/docs#authenticating-with-azure-managed-service-principal-using-github-openid-connect-oidc
//
//nolint:lll // The example URL is longer than the line limit.
func exampleConfigLeaves(g *Generator, code, pulumiYAML string) ([]configLeaf, error) {
	leaves := providerBlockConfigLeaves(g, code)
	other, err := pulumiYAMLConfigLeaves(pulumiYAML, func(key string) bool {
		pkg, _, _ := strings.Cut(key, ":")
		return g.info.Name != "" && (pkg == g.info.Name || pkg == g.pkg.Name().String())
	})
	if err != nil {
		return nil, err
	}
	return append(leaves, other...), nil
}

// providerBlockConfigLeaves returns the config set by this provider's `provider` blocks in code, named and marked
// secret according to the provider's config schema.
func providerBlockConfigLeaves(g *Generator, code string) []configLeaf {
	if g.info.Name == "" {
		return nil
	}
	file, diags := hclsyntax.ParseConfig([]byte(code), "main.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return nil
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	w := configWalker{src: []byte(code), namespace: g.pkg.Name().String(), infos: g.info.Config}
	if g.info.P != nil {
		w.schema = g.info.P.Schema()
	}
	var leaves []configLeaf
	for _, block := range body.Blocks {
		if block.Type != "provider" || len(block.Labels) != 1 || block.Labels[0] != g.info.Name {
			continue
		}
		// An aliased provider block configures an explicit provider resource, not the stack's config.
		if _, aliased := block.Body.Attributes["alias"]; aliased {
			continue
		}
		leaves = append(leaves, w.body(nil, walk.NewSchemaPath(), block.Body, false)...)
	}
	return leaves
}

// configWalker flattens HCL provider config into config leaves, in source order so that the commands follow the
// upstream example.
//
// Each value is tracked by two paths: its Pulumi path, which names the config to set, and its Terraform schema path,
// which looks up its schema and SchemaInfo. Values the schema doesn't describe keep their keys as written.
type configWalker struct {
	// src is the HCL source, for rendering expressions that can't be evaluated.
	src       []byte
	namespace string
	schema    shim.SchemaMap
	infos     map[string]*tfbridge.SchemaInfo
}

func (w configWalker) lookup(sp walk.SchemaPath) (shim.Schema, *tfbridge.SchemaInfo) {
	if w.schema == nil || len(sp) == 0 {
		return nil, nil
	}
	s, info, err := tfbridge.LookupSchemas(sp, w.schema, w.infos)
	if err != nil {
		return nil, nil
	}
	return s, info
}

// field returns the Pulumi name, schema path, and secretness of key within the object or map at sp.
func (w configWalker) field(sp walk.SchemaPath, key string) (string, walk.SchemaPath, bool) {
	fields, infos := w.schema, w.infos
	if len(sp) > 0 {
		fields, infos = nil, nil
		s, info := w.lookup(sp)
		if s != nil && s.Type() == shim.TypeMap {
			r, isObject := s.Elem().(shim.Resource)
			if !isObject {
				// Map keys are data, not field names.
				elem := sp.Element()
				return key, elem, isSecret(w.lookup(elem))
			}
			fields = r.Schema()
			if info != nil {
				infos = info.Fields
			}
		}
	}
	child := sp.GetAttr(key)
	name := key
	if fields != nil {
		name = tfbridge.TerraformToPulumiNameV2(key, fields, infos)
	}
	return name, child, isSecret(w.lookup(child))
}

func (w configWalker) leaf(path resource.PropertyPath, value string, secret bool) []configLeaf {
	return []configLeaf{newConfigLeaf(w.namespace, path, value, secret)}
}

func (w configWalker) body(
	path resource.PropertyPath, sp walk.SchemaPath, body *hclsyntax.Body, secret bool,
) []configLeaf {
	// Attributes are a map, so each entry records where it starts in the source to restore the upstream order.
	type entry struct {
		start  int
		leaves []configLeaf
	}
	var entries []entry
	for key, attr := range body.Attributes {
		name, child, childSecret := w.field(sp, key)
		entries = append(entries, entry{
			attr.SrcRange.Start.Byte,
			w.expr(appendPath(path, name), child, attr.Expr, secret || childSecret),
		})
	}

	blocks := map[string][]*hclsyntax.Block{}
	var types []string
	for _, b := range body.Blocks {
		// Dynamic blocks are generated from expressions that can't be evaluated here.
		if b.Type == "dynamic" {
			continue
		}
		if _, seen := blocks[b.Type]; !seen {
			types = append(types, b.Type)
		}
		blocks[b.Type] = append(blocks[b.Type], b)
	}
	for _, typ := range types {
		name, child, childSecret := w.field(sp, typ)
		p, s := appendPath(path, name), secret || childSecret
		schema, info := w.lookup(child)
		var leaves []configLeaf
		switch {
		case tfbridge.IsMaxItemsOne(schema, info):
			leaves = w.body(p, child.Element(), blocks[typ][0].Body, s)
		case isListSchema(schema):
			for i, b := range blocks[typ] {
				leaves = append(leaves, w.body(appendPath(p, i), child.Element(), b.Body, s)...)
			}
		case len(blocks[typ]) == 1:
			// A single nested block, or a block the schema doesn't know.
			leaves = w.body(p, child, blocks[typ][0].Body, s)
		default:
			for i, b := range blocks[typ] {
				leaves = append(leaves, w.body(appendPath(p, i), child, b.Body, s)...)
			}
		}
		// Repeated blocks are numbered in order, so they are kept together where the first one appears.
		entries = append(entries, entry{blocks[typ][0].Range().Start.Byte, leaves})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].start < entries[j].start })
	var leaves []configLeaf
	for _, e := range entries {
		leaves = append(leaves, e.leaves...)
	}
	return leaves
}

func (w configWalker) expr(
	path resource.PropertyPath, sp walk.SchemaPath, expr hclsyntax.Expression, secret bool,
) []configLeaf {
	switch e := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		if s, info := w.lookup(sp); tfbridge.IsMaxItemsOne(s, info) && len(e.Exprs) == 1 {
			return w.expr(path, sp.Element(), e.Exprs[0], secret)
		}
		var leaves []configLeaf
		for i, item := range e.Exprs {
			leaves = append(leaves, w.expr(appendPath(path, i), sp.Element(), item, secret)...)
		}
		return leaves
	case *hclsyntax.ObjectConsExpr:
		if s, _ := w.lookup(sp); isListSchema(s) {
			sp = sp.Element()
		}
		var leaves []configLeaf
		for _, item := range e.Items {
			key, diags := item.KeyExpr.Value(nil)
			if diags.HasErrors() || !key.Type().Equals(cty.String) || key.IsNull() {
				continue
			}
			name, child, childSecret := w.field(sp, key.AsString())
			leaves = append(leaves, w.expr(appendPath(path, name), child, item.ValueExpr, secret || childSecret)...)
		}
		return leaves
	}

	if v, diags := expr.Value(nil); !diags.HasErrors() && v.IsWhollyKnown() {
		switch {
		case v.IsNull():
			return nil
		case v.Type() == cty.String:
			return w.leaf(path, v.AsString(), secret)
		case v.Type() == cty.Number:
			return w.leaf(path, v.AsBigFloat().Text('f', -1), secret)
		case v.Type() == cty.Bool:
			return w.leaf(path, strconv.FormatBool(v.True()), secret)
		}
	}
	// Values that can't be evaluated here, such as variable references, are marked the same way the converter marks
	// them, so configValueArg renders a placeholder.
	r := expr.Range()
	return w.leaf(path, "TODO: "+string(w.src[r.Start.Byte:r.End.Byte]), secret)
}

func isListSchema(s shim.Schema) bool {
	return s != nil && (s.Type() == shim.TypeList || s.Type() == shim.TypeSet)
}

// isSecret reports whether a property is secret in the generated schema: Terraform `Sensitive`, unless its
// SchemaInfo overrides it.
func isSecret(s shim.Schema, info *tfbridge.SchemaInfo) bool {
	if info != nil && info.Secret != nil {
		return *info.Secret
	}
	return s != nil && s.Sensitive()
}

// appendPath returns path with key added, without sharing path's backing array, since sibling values extend the same
// parent path.
func appendPath(path resource.PropertyPath, key any) resource.PropertyPath {
	return append(slices.Clip(path), key)
}

// pulumiYAMLConfigLeaves returns the config in a converted example's Pulumi.yaml, skipping keys for which skip
// returns true. Nothing is marked secret, since the converter doesn't record which config is sensitive.
func pulumiYAMLConfigLeaves(pulumiYAML string, skip func(key string) bool) ([]configLeaf, error) {
	if pulumiYAML == "" {
		return nil, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(pulumiYAML), &doc); err != nil {
		return nil, fmt.Errorf("parsing converted Pulumi.yaml: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	config := mappingValue(doc.Content[0], "config")
	if config == nil || config.Kind != yaml.MappingNode {
		return nil, nil
	}

	var leaves []configLeaf
	for i := 0; i+1 < len(config.Content); i += 2 {
		key := config.Content[i].Value
		value := mappingValue(config.Content[i+1], "value")
		if value == nil || skip(key) {
			continue
		}
		namespace, name, _ := strings.Cut(key, ":")
		leaves = append(leaves, yamlConfigLeaves(namespace, resource.PropertyPath{name}, value)...)
	}
	return leaves, nil
}

// mappingValue returns the value node for key in a YAML mapping node, or nil if it is absent.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// yamlConfigLeaves flattens a nested config value into `pulumi config set --path` paths and scalar values.
func yamlConfigLeaves(namespace string, path resource.PropertyPath, node *yaml.Node) []configLeaf {
	switch node.Kind {
	case yaml.MappingNode:
		var leaves []configLeaf
		for i := 0; i+1 < len(node.Content); i += 2 {
			leaves = append(leaves,
				yamlConfigLeaves(namespace, appendPath(path, node.Content[i].Value), node.Content[i+1])...)
		}
		return leaves
	case yaml.SequenceNode:
		var leaves []configLeaf
		for i, item := range node.Content {
			leaves = append(leaves, yamlConfigLeaves(namespace, appendPath(path, i), item)...)
		}
		return leaves
	default:
		return []configLeaf{newConfigLeaf(namespace, path, node.Value, false)}
	}
}

var terraformVarRef = regexp.MustCompile(`var\.([A-Za-z0-9_]+)`)

// configValueArg renders a config value as a shell argument. The converter emits "TODO: <expr>" for values it cannot
// translate, such as Terraform variable references; those become a placeholder named after the variable.
func configValueArg(value string) string {
	if expr, ok := strings.CutPrefix(value, "TODO: "); ok {
		if m := terraformVarRef.FindStringSubmatch(expr); m != nil {
			return "<" + m[1] + ">"
		}
		return "<value>"
	}
	return shellQuote(value)
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote returns s unchanged when it needs no quoting in a POSIX shell, and single-quoted otherwise.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// The markers around every translated example in installation docs.
const (
	chooserStart = `{{< chooser language "typescript,python,go,csharp,java,yaml,hcl" >}}` + "\n"
	chooserEnd   = "{{< /chooser >}}\n"
)

func convertExample(g *Generator, code string, exampleNumber int) (string, error) {
	// Make an example to record in the cliConverter.
	converter := g.cliConverter()
	fileName := fmt.Sprintf("configuration-installation-%d", exampleNumber)
	pclExample, err := converter.singleExampleFromHCLToPCL(fileName, code)
	if err != nil {
		return "", err
	}

	// Provider config is the same for every language, so render it once above the language chooser.
	configLeaves, err := exampleConfigLeaves(g, code, pclExample.PulumiYAML)
	if err != nil {
		return "", err
	}
	configCommands := renderConfigSetCommands(configLeaves)

	// If we have provider config but no additional code, we only render the config commands.
	if isEmptyProgram(pclExample.PCL) {
		return configCommands, nil
	}

	langs := genLanguageToSlice(g.language)
	const choosableEnd = "\n{{% /choosable %}}\n"
	exampleContent := chooserStart
	successfulConversion := false

	// Generate each language in turn and mark up the output with the correct Hugo shortcodes.
	for _, lang := range langs {
		choosableStart := fmt.Sprintf("{{%% choosable language %s %%}}\n", lang)

		// Generate language example
		convertedLang, err := converter.singleExampleFromPCLToLanguage(pclExample, lang)
		if err != nil {
			g.warn(err.Error())
		}
		if convertedLang != exampleUnavailable {
			successfulConversion = true
		}
		exampleContent += choosableStart + convertedLang + choosableEnd
	}

	if successfulConversion {
		if configCommands != "" {
			configCommands += "\n"
		}
		return configCommands + exampleContent + chooserEnd, nil
	}
	return configCommands, nil
}

// isEmptyProgram reports whether a converted PCL program declares nothing that would appear in a language example.
// A `terraform { required_providers }` block converts to a PCL `package` block, which no language renders as code, so
// a program of only `package` blocks would render as an empty program in every language.
func isEmptyProgram(pcl string) bool {
	file, diags := hclsyntax.ParseConfig([]byte(pcl), "example.pp", hcl.InitialPos)
	if diags.HasErrors() {
		// Let the language conversion report the problem.
		return strings.TrimSpace(pcl) == ""
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return false
	}
	if len(body.Attributes) > 0 {
		return false
	}
	for _, block := range body.Blocks {
		if block.Type != "package" {
			return false
		}
	}
	return true
}

type titleRemover struct{}

var _ parser.ASTTransformer = titleRemover{}

func (tr titleRemover) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		// The first header we encounter should be the document title.
		header, found := n.(*ast.Heading)
		if !found || header.Level != 1 || !entering {
			return ast.WalkContinue, nil
		}

		parent := n.Parent()
		contract.Assertf(parent != nil, "parent cannot be nil")
		// Removal here is safe, as we want to remove only the first header anyway.
		n.Parent().RemoveChild(parent, header)
		return ast.WalkStop, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk should never error")
}

func removeTitle(
	content []byte,
) ([]byte, error) {
	// Instantiate our transformer
	titleRemover := titleRemover{}
	gm := goldmark.New(
		goldmark.WithExtensions(parse.TFRegistryExtension),
		goldmark.WithParserOptions(parser.WithASTTransformers(
			util.Prioritized(titleRemover, 1000),
		)),
		goldmark.WithRenderer(markdown.NewRenderer()),
	)
	var buf bytes.Buffer
	// Convert parses the source, applies transformers, and renders output to buf
	err := gm.Convert(content, &buf)
	return buf.Bytes(), err
}

type sectionSkipper struct {
	shouldSkipHeader func(headerText string) bool
}

var _ parser.ASTTransformer = sectionSkipper{}

func (t sectionSkipper) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	var sectionsToSkip []ast.Node

	// Walk to find sections that should be skipped.
	// Walk() loses information on subsequent nodes when nodes are removed during the walk, so we only gather them here.
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if section, ok := n.(*section.Section); ok && entering {
			headerText := section.Heading().Lines().Value(source)
			if t.shouldSkipHeader(string(headerText)) {
				parent := section.Parent()
				if parent == nil {
					panic("PARENT IS NIL")
				}
				sectionsToSkip = append(sectionsToSkip, section)
				return ast.WalkSkipChildren, nil
			}
		}
		return ast.WalkContinue, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk should never error")

	// Remove the sections
	for _, section := range sectionsToSkip {
		parent := section.Parent()
		parent.RemoveChild(parent, section)
	}
}

// SkipSectionByHeaderContent removes headers where shouldSkipHeader(header) returns true,
// along with any text under the header.
// content is assumed to be Github flavored markdown when parsing.
//
// shouldSkipHeader is called on the raw header text, like this:
//
//	shouldSkipHeader("My Header")
//
// *not* like this:
//
//	// This is wrong
//	shouldSkipHeader("## My Header\n")
//
// Example of removing the first header and its context
//
//	result := SkipSectionByHeaderContent(byte[](
//	`
//	## First Header
//
//	First content
//
//	## Second Header
//
//	Second content
//	`,
//	func(headerText string) bool) ([]byte, error) {
//		return headerText == "First Header"
//	})
//
// The result will only now contain the second result:
//
//	## Second Header
//
//	Second content
func SkipSectionByHeaderContent(
	content []byte,
	shouldSkipHeader func(headerText string) bool,
) ([]byte, error) {
	// Instantiate our transformer
	sectionSkipper := sectionSkipper{shouldSkipHeader}
	gm := goldmark.New(
		goldmark.WithExtensions(parse.TFRegistryExtension),
		goldmark.WithParserOptions(parser.WithASTTransformers(
			util.Prioritized(sectionSkipper, 902),
		)),
		goldmark.WithRenderer(markdown.NewRenderer()),
	)
	// Goldmark has no notion of YAML front matter and would re-render it as a thematic break and a setext
	// heading, which trimFrontMatter can no longer recognize. Set it aside and restore it verbatim.
	body := trimFrontMatter(content)
	var buf bytes.Buffer
	if frontMatter := content[:len(content)-len(body)]; len(frontMatter) > 0 {
		buf.Write(frontMatter)
		buf.WriteString("\n")
	}
	// Convert parses the source, applies transformers, and renders output to buf
	err := gm.Convert(body, &buf)
	return buf.Bytes(), err
}

// Edit Rule for skipping headers.
func skipSectionHeadersEdit() tfbridge.DocsEdit {
	defaultHeaderSkipRegexps := getDefaultHeadersToSkip()
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			return SkipSectionByHeaderContent(content, func(headerText string) bool {
				for _, header := range defaultHeaderSkipRegexps {
					if header.Match([]byte(headerText)) {
						return true
					}
				}
				return false
			})
		},
		Phase: info.PostCodeTranslation,
	}
}

func getDefaultHeadersToSkip() []*regexp.Regexp {
	defaultHeaderSkipRegexps := []*regexp.Regexp{
		regexp.MustCompile("[Ll]ogging"),
		regexp.MustCompile("[Ll]ogs"),
		regexp.MustCompile("[Tt]esting"),
		regexp.MustCompile("[Dd]evelopment"),
		regexp.MustCompile("[Dd]ebugging"),
		regexp.MustCompile("[Tt]erraform CLI"),
		regexp.MustCompile("[Tt]erraform [Cc]loud"),
		regexp.MustCompile("Delete Protection"),
		regexp.MustCompile("[Cc]ontributing"),
	}
	return defaultHeaderSkipRegexps
}

func getTfVersionsToRemove() []*regexp.Regexp {
	tfVersionsToRemove := []*regexp.Regexp{
		regexp.MustCompile(`(It|This provider) requires( at least)? [tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]?( or later)?.`),
		regexp.MustCompile(`(?s)(For )?[tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]? and (later|earlier):`),
		regexp.MustCompile(`A minimum of [tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]? is recommended.`),
		regexp.MustCompile("[tT]erraform `[v0-9.]+` (and|or) later:"),
	}
	return tfVersionsToRemove
}

func removeTfVersionMentions() tfbridge.DocsEdit {
	tfVersionsToRemove := getTfVersionsToRemove()
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			for _, tfVersion := range tfVersionsToRemove {
				content = tfVersion.ReplaceAll(content, nil)
			}
			return content, nil
		},
		Phase: info.PostCodeTranslation,
	}
}

func getProviderDisplayName(g *Generator) string {
	providerName := g.info.DisplayName
	if providerName != "" {
		return providerName
	}
	// If the provider hasn't set an explicit Display Name, we infer from the package name.
	providerName = g.pkg.Name().String()
	// For display purposes, we'll capitalize the name.
	// This won't always work well - "aws" --> "Aws" isn't necessarily what we want
	// but it's a reasonable fallback option when info.DisplayName isn't set.
	capitalize := cases.Title(language.English)
	return capitalize.String(providerName)
}

func removeEmptySection(title string, contentBytes []byte) ([]byte, error) {
	if !isMarkdownSectionEmpty(title, contentBytes) {
		return contentBytes, nil
	}
	return SkipSectionByHeaderContent(contentBytes, func(headerText string) bool {
		return headerText == title
	})
}

func isMarkdownSectionEmpty(title string, contentBytes []byte) bool {
	gm := goldmark.New(goldmark.WithExtensions(parse.TFRegistryExtension))
	astNode := gm.Parser().Parse(text.NewReader(contentBytes))

	isEmpty := false

	err := ast.Walk(astNode, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if section, ok := n.(*section.Section); ok && entering {
			if section.HasChildren() {
				// A titled section is empty if it has only one child - the title.
				// If the child's text matches the title, the section is empty.
				sectionText := string(section.FirstChild().Lines().Value(contentBytes))
				if section.FirstChild() == section.LastChild() && sectionText == title {
					isEmpty = true
					return ast.WalkStop, nil
				}
			}
		}
		return ast.WalkContinue, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk should never error")

	return isEmpty
}
