package convert

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/terraform/pkg/configs"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

func loadConfigDir(fs afero.Fs, path string) (map[string][]byte, *configs.Module, hcl.Diagnostics) {
	p := configs.NewParser(fs)
	mod, diags := p.LoadConfigDir(path)
	return p.Sources(), mod, diags
}

func convertCtyType(typ cty.Type) string {
	if typ.Equals(cty.Number) {
		return "number"
	}
	if typ.Equals(cty.Bool) {
		return "bool"
	}
	if typ.Equals(cty.String) {
		return "string"
	}
	if typ.IsListType() {
		elementType := convertCtyType(typ.ElementType())
		return fmt.Sprintf("list(%s)", elementType)
	}
	if typ.IsMapType() {
		elementType := convertCtyType(typ.ElementType())
		return fmt.Sprintf("map(%s)", elementType)
	}
	if typ.IsSetType() {
		// handle sets like lists
		elementType := convertCtyType(typ.ElementType())
		return fmt.Sprintf("list(%s)", elementType)
	}
	if typ.IsObjectType() {
		attributeKeys := []string{}
		for attributeKey := range typ.AttributeTypes() {
			attributeKeys = append(attributeKeys, attributeKey)
		}

		// sort the attribute keys so that the resulting object type (as text) is deterministic
		sort.Strings(attributeKeys)
		attributeTypes := typ.AttributeTypes()
		attributes := []string{}
		for _, attributeKey := range attributeKeys {
			attributeType := attributeTypes[attributeKey]
			attributes = append(attributes, fmt.Sprintf("%s=%s", attributeKey, convertCtyType(attributeType)))
		}

		attributePairs := ""
		length := len(attributes)
		for i, attribute := range attributes {
			attributePairs = attributePairs + attribute
			if i < length-1 {
				// add a comma to all pairs but the last one
				attributePairs = attributePairs + ", "
			}
		}

		return fmt.Sprintf("object({%s})", attributePairs)
	}

	// If we got here it's probably the "dynamic type" and we just report back "any"
	return ""
}

// Returns true if the token type is trivia (a comment or new line)
func isTrivia(tokenType hclsyntax.TokenType) bool {
	return tokenType == hclsyntax.TokenComment || tokenType == hclsyntax.TokenNewline
}

// Return the triva before first token index, and after the last token index.
func getTrivaFromIndex(tokens hclsyntax.Tokens, first, last int) (hclwrite.Tokens, hclwrite.Tokens) {
	// Work backwards from first to build up leading trivia
	leading := make(hclwrite.Tokens, 0)
	first = first - 1
	for first >= 0 {
		if isTrivia(tokens[first].Type) {
			leading = append(leading, &hclwrite.Token{
				Type:  tokens[first].Type,
				Bytes: tokens[first].Bytes,
			})
			first = first - 1
		} else {
			break
		}
	}
	// Trim leading new lines
	for len(leading) > 0 {
		end := len(leading) - 1
		if leading[end].Type == hclsyntax.TokenNewline {
			leading = leading[0:end]
		} else {
			break
		}
	}
	// Reverse the list
	for i, j := 0, len(leading)-1; i < j; i, j = i+1, j-1 {
		leading[i], leading[j] = leading[j], leading[i]
	}

	// Now work forwards from last to build up trailing trivia
	trailing := make(hclwrite.Tokens, 0)
	last = last - 1
	for last < len(tokens) {
		if isTrivia(tokens[last].Type) {
			trailing = append(leading, &hclwrite.Token{
				Type:  tokens[last].Type,
				Bytes: tokens[last].Bytes,
			})
			last = last + 1
		} else {
			break
		}
	}

	return leading, trailing
}

// Given a HCL range find the trivia before and after that range
func getTrivia(sources map[string][]byte, r hcl.Range) (hclwrite.Tokens, hclwrite.Tokens) {
	// Load the file referenced in the range
	src, has := sources[r.Filename]
	if !has {
		// This shouldn't ever be hit, "sources" is a list of every file we parsed earlier and ranges should
		// only come from those.
		panic(fmt.Sprintf("Could not read '%s' to parse trivia", r.Filename))
	}
	tokens, _ := hclsyntax.LexConfig(src, r.Filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	// Ignore the diagnostics, we already know this is parsable because we've got the hcl.Range for it

	// Find the index of the first and last token matching the input
	var first, last int
	for i, token := range tokens {
		if token.Range.Start == r.Start {
			first = i
		}
		if token.Range.End == r.End {
			last = i
		}
	}

	return getTrivaFromIndex(tokens, first, last)
}

// Given a HCL range for an attribute expression find the full range for that attribute
func getAttributeRange(sources map[string][]byte, r hcl.Range) hcl.Range {
	// Load the file referenced in the range
	src, has := sources[r.Filename]
	if !has {
		// This shouldn't ever be hit, "sources" is a list of every file we parsed earlier and ranges should
		// only come from those.
		panic(fmt.Sprintf("Could not read '%s' to parse trivia", r.Filename))
	}
	tokens, _ := hclsyntax.LexConfig(src, r.Filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	// Ignore the diagnostics, we already know this is parsable because we've got the hcl.Range for it

	// Find the index of the first token matching the input range
	var first int
	for i, token := range tokens {
		if token.Range.Start == r.Start {
			first = i
			break
		}
	}

	// Now work backwards from first to find the identifier token for this attribute
	first = first - 1
	for first >= 0 {
		if tokens[first].Type == hclsyntax.TokenIdent {
			break
		}
		first = first - 1
	}

	return hcl.Range{
		Filename: r.Filename,
		Start:    tokens[first].Range.Start,
		End:      r.End,
	}
}

// Given a HCL range return the tokens for that range
func getTokensForRange(sources map[string][]byte, r hcl.Range) hclwrite.Tokens {
	// Load the file referenced in the range
	src, has := sources[r.Filename]
	if !has {
		// This shouldn't ever be hit, "sources" is a list of every file we parsed earlier and ranges should
		// only come from those.
		panic(fmt.Sprintf("Could not read '%s' to parse trivia", r.Filename))
	}
	tokens, _ := hclsyntax.LexConfig(src, r.Filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	// Ignore the diagnostics, we already know this is parsable because we've got the hcl.Range for it

	// Find the tokens for this range
	rangeTokens := make(hclwrite.Tokens, 0)
	foundFirst := false
	for _, token := range tokens {
		if token.Range.Start == r.Start {
			foundFirst = true
		}

		if foundFirst {
			rangeTokens = append(rangeTokens, &hclwrite.Token{
				Type:  token.Type,
				Bytes: token.Bytes,
			})
		}

		if token.Range.End == r.End {
			break
		}
	}

	return rangeTokens
}

// Functions need to translate in one of four ways
// 1. The `list` function just gets translated into a tuple
// 2. Just a simple rename, e.g. "file" => "readFile"
// 3. A translation to an std invoke
// 4. Left exactly as is (e.g. length), we assume this for now but really we should probably be explicit.
var tfFunctionRenames = map[string]string{
	"sensitive":  "secret",
	"jsonencode": "toJSON",
	"length":     "length",
}

var tfFunctionStd = map[string]struct {
	token     string
	inputs    []string
	output    string
	paramArgs bool
}{
	"abs": {
		token:  "std:index:abs",
		inputs: []string{"input"},
		output: "result",
	},
	"abspath": {
		token:  "std:index:abspath",
		inputs: []string{"input"},
		output: "result",
	},
	"base64decode": {
		token:  "std:index:base64decode",
		inputs: []string{"input"},
		output: "result",
	},
	"base64encode": {
		token:  "std:index:base64encode",
		inputs: []string{"input"},
		output: "result",
	},
	"base64gzip": {
		token:  "std:index:base64gzip",
		inputs: []string{"input"},
		output: "result",
	},
	"base64sha256": {
		token:  "std:index:base64sha256",
		inputs: []string{"input"},
		output: "result",
	},
	"base64sha512": {
		token:  "std:index:base64sha512",
		inputs: []string{"input"},
		output: "result",
	},
	"basename": {
		token:  "std:index:basename",
		inputs: []string{"input"},
		output: "result",
	},
	"bcrypt": {
		token:  "std:index:bcrypt",
		inputs: []string{"input", "cost"},
		output: "result",
	},
	"ceil": {
		token:  "std:index:ceil",
		inputs: []string{"input"},
		output: "result",
	},
	"chomp": {
		token:  "std:index:chomp",
		inputs: []string{"input"},
		output: "result",
	},
	"concat": {
		token:     "std:index:concat",
		inputs:    []string{"input"},
		output:    "result",
		paramArgs: true,
	},
	/* Currently failing due to: cannot assign expression of type { input: (string, string, string, null,
	string) } to location of type { input: list(output(string) | string) | output(list(string)) } | output({
	input: list(string) }):
	*/
	//
	//	on main.pp line 263:
	//	264:   value = invoke("std:index:compact", {
	//	265:     input = ["a", "", "b", null, "c"]
	//	266:   }).result
	//
	//"compact": {
	//	token:  "std:index:compact",
	//	inputs: []string{"input"},
	//	output: "result",
	//},
	"cidrhost": {
		token:  "std:index:cidrhost",
		inputs: []string{"input", "host"},
		output: "result",
	},
	"cidrnetmask": {
		token:  "std:index:cidrnetmask",
		inputs: []string{"input"},
		output: "result",
	},
	"cidrsubnet": {
		token:  "std:index:cidrsubnet",
		inputs: []string{"input", "newbits", "netnum"},
		output: "result",
	},
	"csvdecode": {
		token:  "std:index:csvdecode",
		inputs: []string{"input"},
		output: "result",
	},
	"dirname": {
		token:  "std:index:dirname",
		inputs: []string{"input"},
		output: "result",
	},
	"endswith": {
		token:  "std:index:endswith",
		inputs: []string{"input", "suffix"},
		output: "result",
	},
	"file": {
		token:  "std:index:file",
		inputs: []string{"input"},
		output: "result",
	},
	"filebase64": {
		token:  "std:index:filebase64",
		inputs: []string{"input"},
		output: "result",
	},
	"filebase64sha256": {
		token:  "std:index:filebase64sha256",
		inputs: []string{"input"},
		output: "result",
	},
	"filebase64sha512": {
		token:  "std:index:filebase64sha512",
		inputs: []string{"input"},
		output: "result",
	},
	"fileexists": {
		token:  "std:index:fileexists",
		inputs: []string{"input"},
		output: "result",
	},
	"filemd5": {
		token:  "std:index:filemd5",
		inputs: []string{"input"},
		output: "result",
	},
	"filesha1": {
		token:  "std:index:filesha1",
		inputs: []string{"input"},
		output: "result",
	},
	"filesha256": {
		token:  "std:index:filesha256",
		inputs: []string{"input"},
		output: "result",
	},
	"filesha512": {
		token:  "std:index:filesha512",
		inputs: []string{"input"},
		output: "result",
	},
	"floor": {
		token:  "std:index:floor",
		inputs: []string{"input"},
		output: "result",
	},
	"indent": {
		token:  "std:index:indent",
		inputs: []string{"spaces", "input"},
		output: "result",
	},
	"join": {
		token:  "std:index:join",
		inputs: []string{"separator", "input"},
		output: "result",
	},
	"log": {
		token:  "std:index:log",
		inputs: []string{"base", "input"},
		output: "result",
	},
	"lower": {
		token:  "std:index:lower",
		inputs: []string{"input"},
		output: "result",
	},
	"max": {
		token:     "std:index:max",
		inputs:    []string{"input"},
		output:    "result",
		paramArgs: true,
	},
	"md5": {
		token:  "std:index:md5",
		inputs: []string{"input"},
		output: "result",
	},
	"min": {
		token:     "std:index:min",
		inputs:    []string{"input"},
		output:    "result",
		paramArgs: true,
	},
	"parseint": {
		token:  "std:index:parseint",
		inputs: []string{"base", "input"},
		output: "result",
	},
	"pathexpand": {
		token:  "std:index:pathexpand",
		inputs: []string{"input"},
		output: "result",
	},
	"pow": {
		token:  "std:index:pow",
		inputs: []string{"base", "exponent"},
		output: "result",
	},
	"range": {
		token:  "std:index:range",
		inputs: []string{"limit", "start", "step"},
		output: "result",
	},
	"replace": {
		token:  "std:index:replace",
		inputs: []string{"replace", "search", "text"},
		output: "result",
	},
	"rsadecrypt": {
		token:  "std:index:rsadecrypt",
		inputs: []string{"cipherText", "key"},
		output: "result",
	},
	"sha1": {
		token:  "std:index:sha1",
		inputs: []string{"input"},
		output: "result",
	},
	"sha256": {
		token:  "std:index:sha256",
		inputs: []string{"input"},
		output: "result",
	},
	"sha512": {
		token:  "std:index:sha512",
		inputs: []string{"input"},
		output: "result",
	},
	"signum": {
		token:  "std:index:signum",
		inputs: []string{"input"},
		output: "result",
	},
	"sort": {
		token:  "std:index:sort",
		inputs: []string{"input"},
		output: "result",
	},
	"split": {
		token:  "std:index:split",
		inputs: []string{"separator", "text"},
		output: "result",
	},
	"startswith": {
		token:  "std:index:startswith",
		inputs: []string{"input", "prefix"},
		output: "result",
	},
	"strrev": {
		token:  "std:index:strrev",
		inputs: []string{"input"},
		output: "result",
	},
	"substr": {
		token:  "std:index:substr",
		inputs: []string{"input", "length", "offset"},
		output: "result",
	},
	"sum": {
		token:  "std:index:sum",
		inputs: []string{"input"},
		output: "result",
	},
	"timeadd": {
		token:  "std:index:timeadd",
		inputs: []string{"duration", "timestamp"},
		output: "result",
	},
	"timecmp": {
		token:  "std:index:timecmp",
		inputs: []string{"timestampa", "timestampb"},
		output: "result",
	},
	"timestamp": {
		token:  "std:index:timestamp",
		inputs: []string{},
		output: "result",
	},
	"title": {
		token:  "std:index:title",
		inputs: []string{"input"},
		output: "result",
	},
	"transpose": {
		token:  "std:index:transpose",
		inputs: []string{"input"},
		output: "result",
	},
	"trim": {
		token:  "std:index:trim",
		inputs: []string{"input", "cutset"},
		output: "result",
	},
	"trimprefix": {
		token:  "std:index:trimprefix",
		inputs: []string{"input", "prefix"},
		output: "result",
	},
	"trimspace": {
		token:  "std:index:trimspace",
		inputs: []string{"input"},
		output: "result",
	},
	"trimsuffix": {
		token:  "std:index:trimsuffix",
		inputs: []string{"input", "suffix"},
		output: "result",
	},
	"upper": {
		token:  "std:index:upper",
		inputs: []string{"input"},
		output: "result",
	},
	"urlencode": {
		token:  "std:index:urlencode",
		inputs: []string{"input"},
		output: "result",
	},
	"uuid": {
		token:  "std:index:uuid",
		inputs: []string{},
		output: "result",
	},
}

func convertFunctionCallExpr(sources map[string][]byte,
	scopes *scopes, fullyQualifiedPath string, call *hclsyntax.FunctionCallExpr,
) hclwrite.Tokens {
	args := []hclwrite.Tokens{}
	for _, arg := range call.Args {
		args = append(args, convertExpression(sources, scopes, "", arg))
	}

	// First see if this is `list`
	if call.Name == "list" {
		listTokens := hclwrite.Tokens{makeToken(hclsyntax.TokenOBrack, "[")}
		first := true
		for _, arg := range args {
			if !first {
				listTokens = append(listTokens, makeToken(hclsyntax.TokenComma, ", "))
			}
			first = false
			listTokens = append(listTokens, arg...)
		}
		listTokens = append(listTokens, makeToken(hclsyntax.TokenCBrack, "]"))
		return listTokens
	}

	// Next see if this is a rename
	if newName, has := tfFunctionRenames[call.Name]; has {
		return hclwrite.TokensForFunctionCall(newName, args...)
	}

	// Next see if it's mapped to a PCL invoke
	if invoke, has := tfFunctionStd[call.Name]; has {
		invokeArgs := make([]hclwrite.ObjectAttrTokens, 0)
		if invoke.paramArgs && len(args) != 1 {
			if len(invoke.inputs) != 1 {
				panic(fmt.Sprintf("Got %d inputs to params style function %s", len(invoke.inputs), invoke.token))
			}

			listTokens := hclwrite.Tokens{makeToken(hclsyntax.TokenOBrack, "[")}
			first := true
			for _, arg := range args {
				if !first {
					listTokens = append(listTokens, makeToken(hclsyntax.TokenComma, ","))
				}
				first = false
				listTokens = append(listTokens, arg...)
			}
			listTokens = append(listTokens, makeToken(hclsyntax.TokenCBrack, "]"))

			invokeArgs = append(invokeArgs, hclwrite.ObjectAttrTokens{
				Name:  hclwrite.TokensForIdentifier(invoke.inputs[0]),
				Value: listTokens,
			})

		} else {
			for i, arg := range args {
				if len(invoke.inputs) <= i {
					panic(fmt.Sprintf("Got argument %d to %s", i, call.Name))
				}

				invokeArgs = append(invokeArgs, hclwrite.ObjectAttrTokens{
					Name:  hclwrite.TokensForIdentifier(invoke.inputs[i]),
					Value: arg,
				})
			}
		}
		token := hclwrite.TokensForValue(cty.StringVal(invoke.token))
		call := hclwrite.TokensForFunctionCall("invoke", token, hclwrite.TokensForObject(invokeArgs))

		if invoke.output == "" {
			return call
		}

		// Add an attribute access to get the output
		call = append(call, makeToken(hclsyntax.TokenDot, "."))
		call = append(call, makeToken(hclsyntax.TokenIdent, invoke.output))
		return call
	}

	// Finally just return it as not yet implemented
	buffer := bytes.NewBufferString("")
	_, err := getTokensForRange(sources, call.Range()).WriteTo(buffer)
	contract.AssertNoErrorf(err, "Failed to write tokens for range %v", call.Range())
	text := cty.StringVal(buffer.String())
	return hclwrite.TokensForFunctionCall("notImplemented", hclwrite.TokensForValue(text))
}

func convertTupleConsExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.TupleConsExpr,
) hclwrite.Tokens {
	elems := []hclwrite.Tokens{}
	for _, expr := range expr.Exprs {
		elems = append(elems, convertExpression(sources, scopes, "", expr))
	}
	tokens := hclwrite.TokensForTuple(elems)
	leading, trailing := getTrivia(sources, expr.SrcRange)
	return append(append(leading, tokens...), trailing...)
}

// appendPath appends a part to a fully quailifed dot-separated path. If the root is "" then append returns
// "". If the part is "" then append returns root.
func appendPath(root, part string) string {
	if root == "" || part == "" {
		return ""
	}
	return root + "." + part
}

// matchStaticString returns a literal string if the expression is a static string or identifier, else nil
func matchStaticString(expr hclsyntax.Expression) *string {
	switch expr := expr.(type) {
	case *hclsyntax.ObjectConsKeyExpr:
		return matchStaticString(expr.Wrapped)
	case *hclsyntax.LiteralValueExpr:
		if expr.Val.Type() != cty.String {
			return nil
		}
		s := expr.Val.AsString()
		return &s
	case *hclsyntax.ScopeTraversalExpr:
		if len(expr.Traversal) != 1 {
			return nil
		}
		if root, ok := expr.Traversal[0].(hcl.TraverseRoot); ok {
			s := root.Name
			return &s
		}
	}
	return nil
}

func convertObjectConsExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.ObjectConsExpr,
) hclwrite.Tokens {
	items := []hclwrite.ObjectAttrTokens{}
	for _, item := range expr.Items {
		// Keys _might_ need renaming if we're translating for an object type, we can do this if it's
		// statically known and we know our current path
		var nameTokens hclwrite.Tokens
		if fullyQualifiedPath != "" {
			name := matchStaticString(item.KeyExpr)
			if name != nil {
				fullyQualifiedPath = appendPath(fullyQualifiedPath, *name)
				nameTokens = hclwrite.TokensForIdentifier(scopes.pulumiName(fullyQualifiedPath))
			}
		}
		// If we can't statically determine the name, we can't rename it, so just convert the expression.
		if nameTokens == nil {
			nameTokens = convertExpression(sources, scopes, "", item.KeyExpr)
			fullyQualifiedPath = ""
		}
		valueTokens := convertExpression(sources, scopes, fullyQualifiedPath, item.ValueExpr)
		items = append(items, hclwrite.ObjectAttrTokens{
			Name:  nameTokens,
			Value: valueTokens,
		})
	}
	return hclwrite.TokensForObject(items)
}

func convertObjectConsKeyExpr(sources map[string][]byte,
	scopes *scopes, fullyQualifiedPath string, expr *hclsyntax.ObjectConsKeyExpr,
) hclwrite.Tokens {
	// Seems we can just ignore ForceNonLiteral here
	return convertExpression(sources, scopes, fullyQualifiedPath, expr.Wrapped)
}

func convertLiteralValueExpr(sources map[string][]byte, expr *hclsyntax.LiteralValueExpr) hclwrite.Tokens {
	leading, trailing := getTrivia(sources, expr.SrcRange)
	value := hclwrite.TokensForValue(expr.Val)

	tokens := leading
	tokens = append(tokens, value...)
	tokens = append(tokens, trailing...)
	return tokens
}

func makeToken(typ hclsyntax.TokenType, str string) *hclwrite.Token {
	return &hclwrite.Token{
		Type:  typ,
		Bytes: []byte(str),
	}
}

func convertTemplateWrapExpr(sources map[string][]byte,
	scopes *scopes, fullyQualifiedPath string, expr *hclsyntax.TemplateWrapExpr,
) hclwrite.Tokens {
	tokens := []*hclwrite.Token{}
	tokens = append(tokens, makeToken(hclsyntax.TokenOQuote, "\""))
	tokens = append(tokens, makeToken(hclsyntax.TokenTemplateInterp, "${"))
	tokens = append(tokens, convertExpression(sources, scopes, "", expr.Wrapped)...)
	tokens = append(tokens, makeToken(hclsyntax.TokenTemplateSeqEnd, "}"))
	tokens = append(tokens, makeToken(hclsyntax.TokenCQuote, "\""))
	return tokens
}

func convertTemplateExpr(sources map[string][]byte,
	scopes *scopes, fullyQualifiedPath string, expr *hclsyntax.TemplateExpr,
) hclwrite.Tokens {
	tokens := []*hclwrite.Token{}
	tokens = append(tokens, makeToken(hclsyntax.TokenOQuote, "\""))
	for _, part := range expr.Parts {
		// If it's a literal then we can just write it to the string directly, else we need to wrap it in a
		// ${} block.
		if lit, ok := part.(*hclsyntax.LiteralValueExpr); ok {
			if lit.Val.Type() == cty.String {
				// Strings get written directly without their surrounding quotes
				strtoks := hclwrite.TokensForValue(lit.Val)
				// strip the first and last token (")
				if len(strtoks) > 0 {
					strtoks = strtoks[1 : len(strtoks)-1]
				}

				tokens = append(tokens, strtoks...)
			} else {
				// Other values can be written as is
				tokens = append(tokens, hclwrite.TokensForValue(lit.Val)...)
			}
		} else {
			tokens = append(tokens, makeToken(hclsyntax.TokenTemplateInterp, "${"))
			tokens = append(tokens, convertExpression(sources, scopes, "", part)...)
			tokens = append(tokens, makeToken(hclsyntax.TokenTemplateSeqEnd, "}"))
		}
	}
	tokens = append(tokens, makeToken(hclsyntax.TokenCQuote, "\""))
	return tokens
}

func camelCaseName(name string) string {
	name = tfbridge.TerraformToPulumiNameV2(name, nil, nil)
	name = strings.ToLower(string(rune(name[0]))) + name[1:]
	return name
}

func rewriteTraversal(scopes *scopes, fullyQualifiedPath string, traversal hcl.Traversal) hcl.Traversal {
	// We need to rewrite traversals, because we don't have the same top level variable names as terraform.
	if len(traversal) == 0 {
		return traversal
	}

	newTraversal := make([]hcl.Traverser, 0)

	var maybeFirstAttr *hcl.TraverseAttr
	if len(traversal) > 1 {
		if attr, ok := traversal[1].(hcl.TraverseAttr); ok {
			maybeFirstAttr = &attr
		}
	}
	var maybeSecondAttr *hcl.TraverseAttr
	if len(traversal) > 2 {
		if attr, ok := traversal[2].(hcl.TraverseAttr); ok {
			maybeSecondAttr = &attr
		}
	}

	if root, ok := traversal[0].(hcl.TraverseRoot); ok {
		if root.Name == "var" && maybeFirstAttr != nil {
			// This is a lookup of a var etc, we need to rewrite this traversal such that the root is now the
			// pulumi config value instead.
			newName := scopes.getOrAddPulumiName("var."+maybeFirstAttr.Name, "", "Config")
			newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
			newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[2:])...)
		} else if root.Name == "local" && maybeFirstAttr != nil {
			// This is a lookup of a local etc, we need to rewrite this traversal such that the root is now the
			// pulumi local value instead.
			newName := scopes.getOrAddPulumiName("local."+maybeFirstAttr.Name, "their", "")
			newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
			newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[2:])...)
		} else if root.Name == "data" && maybeFirstAttr != nil && maybeSecondAttr != nil {
			// This is a lookup of a data resources etc, we need to rewrite this traversal such that the root is now the
			// pulumi invoked value instead.
			suffix := camelCaseName(maybeFirstAttr.Name)
			newName := scopes.getOrAddPulumiName("data."+maybeFirstAttr.Name+"."+maybeSecondAttr.Name, "", "data"+suffix)
			newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
			newTraversal = append(newTraversal, rewriteTraversal(scopes, "data."+maybeFirstAttr.Name, traversal[3:])...)
		} else if root.Name == "count" && maybeFirstAttr != nil {
			if maybeFirstAttr.Name == "index" && scopes.countIndex != nil {
				newTraversal = append(newTraversal, scopes.countIndex...)
				newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[2:])...)
			} else {
				// We didn't have a count_index set but saw count.index!
				contract.Failf("count.index seen during expression conversion, but index scope not set")
			}
		} else if root.Name == "each" && maybeFirstAttr != nil {
			if maybeFirstAttr.Name == "key" {
				if scopes.eachKey != nil {
					newTraversal = append(newTraversal, scopes.eachKey...)
					newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[2:])...)
				} else {
					contract.Failf("each.key seen during expression conversion, but each scope not set")
				}
			}

			if maybeFirstAttr.Name == "value" {
				if scopes.eachValue != nil {
					newTraversal = append(newTraversal, scopes.eachValue...)
					newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[2:])...)
				} else {
					contract.Failf("each.value seen during expression conversion, but each scope not set")
				}
			}
		} else if maybeFirstAttr != nil {
			// This is a lookup of a resource or an attribute lookup on a local variable etc, we need to
			// rewrite this traversal such that the root is now the pulumi invoked value instead.

			// First see if this is a resource
			newName := scopes.lookup(root.Name + "." + maybeFirstAttr.Name)
			if newName != "" {
				// Looks like this is a resource because a local variable would not be recorded in scopes with a "." in it.
				newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
				newTraversal = append(newTraversal, rewriteTraversal(scopes, root.Name, traversal[2:])...)
			} else {
				// This is either a local variable or a resource we haven't seen yet. First check if this is a local variable
				newName := scopes.lookup(root.Name)
				if newName != "" {
					// Looks like this is a local variable, just rewrite the rest of the traversal
					newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
					newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[1:])...)
				} else {
					// We don't know what this is, so lets assume it's an unknown resource (we shouldn't ever have unknown locals)
					newName = scopes.getOrAddPulumiName(root.Name+"."+maybeFirstAttr.Name, "", camelCaseName(root.Name))
					newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
					newTraversal = append(newTraversal, rewriteTraversal(scopes, root.Name, traversal[2:])...)
				}
			}
		} else {
			// This is a lookup of a variable, look it up and use it else just us the name given
			newName := scopes.lookup(root.Name)
			if newName != "" {
				newTraversal = append(newTraversal, hcl.TraverseRoot{Name: newName})
				newTraversal = append(newTraversal, rewriteTraversal(scopes, "", traversal[1:])...)
			} else {
				// This will be an object key or an undeclared variable, just return those as is
				newTraversal = append(newTraversal, traversal...)
			}
		}
	} else if attr, ok := traversal[0].(hcl.TraverseAttr); ok {
		// An attribute look up, we need to know the type path of the traversal so far to resolve this correctly
		var name string
		if fullyQualifiedPath != "" {
			fullyQualifiedPath = appendPath(fullyQualifiedPath, attr.Name)
			name = scopes.pulumiName(fullyQualifiedPath)
		} else {
			name = tfbridge.TerraformToPulumiNameV2(attr.Name, nil, nil)
		}

		newTraversal = append(newTraversal, hcl.TraverseAttr{Name: name})
		newTraversal = append(newTraversal, rewriteTraversal(scopes, fullyQualifiedPath, traversal[1:])...)
	} else if index, ok := traversal[0].(hcl.TraverseIndex); ok {
		// Index just translates as is
		newTraversal = append(newTraversal, hcl.TraverseIndex{Key: index.Key})
		newTraversal = append(newTraversal, rewriteTraversal(scopes, fullyQualifiedPath, traversal[1:])...)
	} else {
		panic(fmt.Sprintf("Traverser %T not handled", traversal[0]))
	}

	return newTraversal
}

func convertScopeTraversalExpr(
	scopes *scopes, fullyQualifiedPath string, expr *hclsyntax.ScopeTraversalExpr,
) hclwrite.Tokens {
	return hclwrite.TokensForTraversal(rewriteTraversal(scopes, fullyQualifiedPath, expr.Traversal))
}

func convertRelativeTraversalExpr(
	sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.RelativeTraversalExpr,
) hclwrite.Tokens {
	tokens := convertExpression(sources, scopes, "", expr.Source)
	tokens = append(tokens, hclwrite.TokensForTraversal(rewriteTraversal(scopes, fullyQualifiedPath, expr.Traversal))...)
	return tokens
}

func convertBinaryOpExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.BinaryOpExpr,
) hclwrite.Tokens {
	tokens := convertExpression(sources, scopes, fullyQualifiedPath, expr.LHS)
	switch expr.Op {
	case hclsyntax.OpLogicalOr:
		tokens = append(tokens, makeToken(hclsyntax.TokenOr, "||"))
	case hclsyntax.OpLogicalAnd:
		tokens = append(tokens, makeToken(hclsyntax.TokenAnd, "&&"))
	case hclsyntax.OpEqual:
		tokens = append(tokens, makeToken(hclsyntax.TokenEqualOp, "=="))
	case hclsyntax.OpNotEqual:
		tokens = append(tokens, makeToken(hclsyntax.TokenNotEqual, "!="))
	case hclsyntax.OpGreaterThan:
		tokens = append(tokens, makeToken(hclsyntax.TokenGreaterThan, ">"))
	case hclsyntax.OpGreaterThanOrEqual:
		tokens = append(tokens, makeToken(hclsyntax.TokenGreaterThanEq, ">="))
	case hclsyntax.OpLessThan:
		tokens = append(tokens, makeToken(hclsyntax.TokenLessThan, "<"))
	case hclsyntax.OpLessThanOrEqual:
		tokens = append(tokens, makeToken(hclsyntax.TokenLessThanEq, "<="))
	case hclsyntax.OpAdd:
		tokens = append(tokens, makeToken(hclsyntax.TokenPlus, "+"))
	case hclsyntax.OpSubtract:
		tokens = append(tokens, makeToken(hclsyntax.TokenMinus, "-"))
	case hclsyntax.OpMultiply:
		tokens = append(tokens, makeToken(hclsyntax.TokenStar, "*"))
	case hclsyntax.OpDivide:
		tokens = append(tokens, makeToken(hclsyntax.TokenSlash, "/"))
	case hclsyntax.OpModulo:
		tokens = append(tokens, makeToken(hclsyntax.TokenPercent, "%"))
	default:
		contract.Failf("unknown binary operation: %T", expr)
	}
	tokens = append(tokens, convertExpression(sources, scopes, fullyQualifiedPath, expr.RHS)...)
	return tokens
}

func convertUnaryOpExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.UnaryOpExpr,
) hclwrite.Tokens {
	var tokens hclwrite.Tokens
	switch expr.Op {
	case hclsyntax.OpLogicalNot:
		tokens = hclwrite.Tokens{makeToken(hclsyntax.TokenBang, "!")}
	case hclsyntax.OpNegate:
		tokens = hclwrite.Tokens{makeToken(hclsyntax.TokenMinus, "-")}
	default:
		contract.Failf("unknown unary operation: %T", expr)
	}
	tokens = append(tokens, convertExpression(sources, scopes, fullyQualifiedPath, expr.Val)...)
	return tokens
}

func convertForExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.ForExpr,
) hclwrite.Tokens {
	if expr.Group {
		contract.Failf("ForExpr.Group is not handled")
	}

	// The collection doesn't yet have access to the key/value scopes
	collTokens := convertExpression(sources, scopes, "", expr.CollExpr)

	// TODO: We should ensure key and value vars are unique
	locals := map[string]string{
		expr.ValVar: camelCaseName(expr.ValVar),
	}
	if expr.KeyVar != "" {
		locals[expr.KeyVar] = camelCaseName(expr.KeyVar)
	}
	scopes.push(locals)

	keyTokens := convertExpression(sources, scopes, "", expr.KeyExpr)
	valueTokens := convertExpression(sources, scopes, "", expr.ValExpr)
	condTokens := convertExpression(sources, scopes, "", expr.CondExpr)

	scopes.pop()

	// Translate to either a tuple or object expression
	// ForExpr = forTupleExpr | forObjectExpr;
	// forTupleExpr = "[" forIntro Expression forCond? "]";
	// forObjectExpr = "{" forIntro Expression "=>" Expression "..."? forCond? "}";
	// forIntro = "for" Identifier ("," Identifier)? "in" Expression ":";
	// forCond = "if" Expression;
	var tokens hclwrite.Tokens
	if keyTokens == nil {
		// This is a forTupleExpr
		tokens = hclwrite.Tokens{makeToken(hclsyntax.TokenOBrack, "[")}
	} else {
		// This is a forObjectExpr
		tokens = hclwrite.Tokens{makeToken(hclsyntax.TokenOBrace, "{")}
	}

	// Write the intro
	tokens = append(tokens, makeToken(hclsyntax.TokenIdent, "for"))
	if locals[expr.KeyVar] != "" {
		tokens = append(tokens, makeToken(hclsyntax.TokenIdent, locals[expr.KeyVar]))
		tokens = append(tokens, makeToken(hclsyntax.TokenComma, ","))
	}
	tokens = append(tokens, makeToken(hclsyntax.TokenIdent, locals[expr.ValVar]))
	tokens = append(tokens, makeToken(hclsyntax.TokenIdent, "in"))
	tokens = append(tokens, collTokens...)
	tokens = append(tokens, makeToken(hclsyntax.TokenColon, ":"))

	// Write the key part (if present)
	if keyTokens != nil {
		tokens = append(tokens, keyTokens...)
		tokens = append(tokens, makeToken(hclsyntax.TokenFatArrow, "=>"))
	}

	// Write the value part
	tokens = append(tokens, valueTokens...)

	// Write the conditional part (if present)
	if condTokens != nil {
		tokens = append(tokens, makeToken(hclsyntax.TokenIdent, "if"))
		tokens = append(tokens, condTokens...)
	}

	// Write the end
	if keyTokens == nil {
		// for a forTupleExpr
		tokens = append(tokens, makeToken(hclsyntax.TokenCBrack, "]"))
	} else {
		// for a forObjectExpr
		tokens = append(tokens, makeToken(hclsyntax.TokenCBrace, "}"))
	}

	return tokens
}

func convertIndexExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.IndexExpr,
) hclwrite.Tokens {
	collection := convertExpression(sources, scopes, fullyQualifiedPath, expr.Collection)
	key := convertExpression(sources, scopes, "", expr.Key)

	tokens := collection
	tokens = append(tokens, makeToken(hclsyntax.TokenOBrack, "["))
	tokens = append(tokens, key...)
	tokens = append(tokens, makeToken(hclsyntax.TokenCBrack, "]"))
	return tokens
}

func convertSplatExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.SplatExpr,
) hclwrite.Tokens {
	source := convertExpression(sources, scopes, "", expr.Source)
	each := convertExpression(sources, scopes, "", expr.Each)

	tokens := source
	tokens = append(tokens, makeToken(hclsyntax.TokenOBrack, "["))
	tokens = append(tokens, makeToken(hclsyntax.TokenStar, "*"))
	tokens = append(tokens, makeToken(hclsyntax.TokenCBrack, "]"))
	tokens = append(tokens, each...)
	return tokens
}

func convertAnonSymbolExpr(scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.AnonSymbolExpr,
) hclwrite.Tokens {
	return hclwrite.Tokens{}
}

func convertConditionalExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.ConditionalExpr,
) hclwrite.Tokens {
	condition := convertExpression(sources, scopes, "", expr.Condition)
	trueResult := convertExpression(sources, scopes, "", expr.TrueResult)
	falseResult := convertExpression(sources, scopes, "", expr.FalseResult)

	tokens := condition
	tokens = append(tokens, makeToken(hclsyntax.TokenQuestion, "?"))
	tokens = append(tokens, trueResult...)
	tokens = append(tokens, makeToken(hclsyntax.TokenColon, ":"))
	tokens = append(tokens, falseResult...)
	return tokens
}

func convertParenthesesExpr(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr *hclsyntax.ParenthesesExpr,
) hclwrite.Tokens {
	tokens := hclwrite.Tokens{makeToken(hclsyntax.TokenOParen, "(")}
	tokens = append(tokens, convertExpression(sources, scopes, "", expr.Expression)...)
	tokens = append(tokens, makeToken(hclsyntax.TokenCParen, ")"))
	return tokens
}

func convertExpression(sources map[string][]byte, scopes *scopes,
	fullyQualifiedPath string, expr hcl.Expression,
) hclwrite.Tokens {
	if expr == nil {
		return nil
	}

	switch expr := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		return convertTupleConsExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.ObjectConsExpr:
		return convertObjectConsExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.ObjectConsKeyExpr:
		return convertObjectConsKeyExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.FunctionCallExpr:
		return convertFunctionCallExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.LiteralValueExpr:
		return convertLiteralValueExpr(sources, expr)
	case *hclsyntax.TemplateExpr:
		return convertTemplateExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.ScopeTraversalExpr:
		return convertScopeTraversalExpr(scopes, fullyQualifiedPath, expr)
	case *hclsyntax.BinaryOpExpr:
		return convertBinaryOpExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.UnaryOpExpr:
		return convertUnaryOpExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.ForExpr:
		return convertForExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.IndexExpr:
		return convertIndexExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.RelativeTraversalExpr:
		return convertRelativeTraversalExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.SplatExpr:
		return convertSplatExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.AnonSymbolExpr:
		return convertAnonSymbolExpr(scopes, fullyQualifiedPath, expr)
	case *hclsyntax.TemplateWrapExpr:
		return convertTemplateWrapExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.ConditionalExpr:
		return convertConditionalExpr(sources, scopes, fullyQualifiedPath, expr)
	case *hclsyntax.ParenthesesExpr:
		return convertParenthesesExpr(sources, scopes, fullyQualifiedPath, expr)
	}
	contract.Failf("Couldn't convert expression: %T", expr)
	return nil
}

type bodyAttrTokens struct {
	Line   int // This is so we can sort these in source order
	Name   string
	Trivia hclwrite.Tokens // Leading trivia
	Value  hclwrite.Tokens // The value _and_ trailing trivia
}

func tokensForObject(ts []bodyAttrTokens) hclwrite.Tokens {
	attrs := make([]hclwrite.ObjectAttrTokens, 0, len(ts))
	for _, attr := range ts {
		name := append(attr.Trivia, hclwrite.TokensForIdentifier(attr.Name)...)
		attrs = append(attrs, hclwrite.ObjectAttrTokens{
			Name:  name,
			Value: attr.Value,
		})
	}
	return hclwrite.TokensForObject(attrs)
}

type bodyAttrsTokens []bodyAttrTokens

func (ts bodyAttrsTokens) Len() int      { return len(ts) }
func (ts bodyAttrsTokens) Swap(i, j int) { ts[i], ts[j] = ts[j], ts[i] }
func (ts bodyAttrsTokens) Less(i, j int) bool {
	return ts[i].Line < ts[j].Line
}

func (ts bodyAttrsTokens) Line() int {
	if len(ts) == 0 {
		return 0
	}
	line := math.MaxInt32
	for _, attr := range ts {
		if attr.Line < line {
			line = attr.Line
		}
	}
	return line
}

// Convert a hcl.Body treating sub-bodies as attributes
func convertBody(sources map[string][]byte, scopes *scopes, fullyQualifiedPath string, body hcl.Body) bodyAttrsTokens {
	contract.Assertf(fullyQualifiedPath != "", "fullyQualifiedPath should not be empty")

	// We want to exclude any hidden blocks and attributes, and the only way to do that with hcl.Body is to
	// give it a schema. JustAttributes() will return all non-hidden attributes, but will error if there's
	// any blocks, and there's no equivalent to get non-hidden attributes and blocks.
	hclSchema := &hcl.BodySchema{}
	// The `body` passed in here _should_ be a hclsyntax.Body. That's currently the only way to just iterate
	// all the raw blocks of a hcl.Body.
	synbody, ok := body.(*hclsyntax.Body)
	contract.Assertf(ok, "%T was not a hclsyntax.Body", body)
	for _, block := range synbody.Blocks {
		if block.Type != "dynamic" {
			hclSchema.Blocks = append(hclSchema.Blocks, hcl.BlockHeaderSchema{Type: block.Type})
		} else {
			// Dynamic blocks have labels on them, we need to tell the schema that's ok.
			hclSchema.Blocks = append(hclSchema.Blocks, hcl.BlockHeaderSchema{
				Type:       block.Type,
				LabelNames: block.Labels,
			})
		}
	}
	for _, attr := range synbody.Attributes {
		hclSchema.Attributes = append(hclSchema.Attributes, hcl.AttributeSchema{Name: attr.Name})
	}
	content, diagnostics := body.Content(hclSchema)
	contract.Assertf(len(diagnostics) == 0, "diagnostics was not empty: %s", diagnostics.Error())

	newAttributes := make(bodyAttrsTokens, 0)

	// If we see blocks we turn those into lists (unless maxItems==1)
	blockLists := make(map[string][]bodyAttrsTokens)
	for _, block := range content.Blocks {
		if block.Type == "dynamic" {
			eachVar := scopes.generateUniqueName("entry", "", "")
			dynamicTokens := hclwrite.Tokens{makeToken(hclsyntax.TokenOBrack, "[")}
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenIdent, "for"))
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenIdent, eachVar))
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenIdent, "in"))
			dynamicBody, ok := block.Body.(*hclsyntax.Body)
			if !ok {
				continue
			}

			forEachAttr, hasForEachAttr := dynamicBody.Attributes["for_each"]
			if !hasForEachAttr {
				continue
			}

			// wrap the collection expression into `entries(collection)` so that each entry has key and value
			forEachExprTokens := convertExpression(sources, scopes, fullyQualifiedPath, forEachAttr.Expr)
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenIdent, "entries"))
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenOParen, "("))
			dynamicTokens = append(dynamicTokens, forEachExprTokens...)
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenCParen, ")"))
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenColon, ":"))

			bodyTokens := hclwrite.Tokens{makeToken(hclsyntax.TokenIdent, "{}")}
			attributeName := scopes.pulumiName(block.Labels[0])
			for _, innerBlock := range dynamicBody.Blocks {
				if innerBlock.Type == "content" {
					scopes.push(map[string]string{
						block.Labels[0]: eachVar,
					})
					contentBody := convertBody(sources, scopes, fullyQualifiedPath+"."+attributeName, innerBlock.Body)
					bodyTokens = tokensForObject(contentBody)
					scopes.pop()
				}
			}

			dynamicTokens = append(dynamicTokens, bodyTokens...)
			dynamicTokens = append(dynamicTokens, makeToken(hclsyntax.TokenCBrack, "]"))

			newAttributes = append(newAttributes, bodyAttrTokens{
				Name:  attributeName,
				Value: dynamicTokens,
			})
			continue
		}

		blockPath := appendPath(fullyQualifiedPath, block.Type)
		schema := scopes.getTerraformSchema(blockPath)
		name := scopes.pulumiName(blockPath)

		isList := false
		if s, ok := schema.(shim.Schema); ok {
			isList = s.MaxItems() != 1
		}

		if !isList {
			// This is a block attribute, not a list
			newAttributes = append(newAttributes, bodyAttrTokens{
				Line:  block.DefRange.Start.Line,
				Name:  name,
				Value: tokensForObject(convertBody(sources, scopes, blockPath, block.Body)),
			})
		} else {
			list := blockLists[name]
			list = append(list, convertBody(sources, scopes, blockPath, block.Body))
			blockLists[name] = list
		}
	}

	for name, items := range blockLists {
		listTokens := hclwrite.Tokens{makeToken(hclsyntax.TokenOBrack, "[")}
		first := true
		line := math.MaxInt32
		for _, item := range items {
			if !first {
				listTokens = append(listTokens, makeToken(hclsyntax.TokenComma, ","))
			}
			first = false
			listTokens = append(listTokens, tokensForObject(item)...)
			if item.Line() < line {
				line = item.Line()
			}
		}
		listTokens = append(listTokens, makeToken(hclsyntax.TokenCBrack, "]"))

		newAttributes = append(newAttributes, bodyAttrTokens{
			Line:  line,
			Name:  name,
			Value: listTokens,
		})
	}

	for _, attr := range content.Attributes {
		attrPath := appendPath(fullyQualifiedPath, attr.Name)
		name := scopes.pulumiName(attrPath)

		leading, trailing := getTrivia(sources, getAttributeRange(sources, attr.Expr.Range()))
		expr := convertExpression(sources, scopes, attrPath, attr.Expr)
		expr = append(expr, trailing...)
		newAttributes = append(newAttributes, bodyAttrTokens{
			Line:   attr.Range.Start.Line,
			Name:   name,
			Trivia: leading,
			Value:  expr,
		})
	}
	sort.Sort(newAttributes)
	return newAttributes
}

func convertVariable(sources map[string][]byte, scopes *scopes,
	variable *configs.Variable) (hclwrite.Tokens, *hclwrite.Block, hclwrite.Tokens) {
	pulumiName := scopes.roots["var."+variable.Name]
	labels := []string{pulumiName}

	pulumiType := convertCtyType(variable.Type)
	if pulumiType != "" {
		labels = append(labels, pulumiType)
	} else if !variable.Default.IsNull() {
		// If we don't have an explicit type but we do have a default value, use it's type
		pulumiType = convertCtyType(variable.Default.Type())
		if pulumiType != "" {
			labels = append(labels, pulumiType)
		}
	}

	block := hclwrite.NewBlock("config", labels)
	blockBody := block.Body()
	if !variable.Default.IsNull() {
		blockBody.SetAttributeValue("default", variable.Default)
	}
	if variable.DescriptionSet {
		blockBody.SetAttributeValue("description", cty.StringVal(variable.Description))
	}
	if variable.NullableSet {
		blockBody.SetAttributeValue("nullable", cty.BoolVal(variable.Nullable))
	}
	leading, trailing := getTrivia(sources, variable.DeclRange)
	return leading, block, trailing
}

func impliedProvider(typeName string) string {
	if under := strings.Index(typeName, "_"); under != -1 {
		typeName = typeName[:under]
	}
	return typeName
}

// Best guess at converting a tf type to a pulumi type
func impliedToken(typeName string) string {
	if under := strings.Index(typeName, "_"); under != -1 {
		provider := typeName[:under]
		typeName = typeName[under+1:]
		return fmt.Sprintf("%s:index:%s", provider, camelCaseName(typeName))
	}
	return camelCaseName(typeName)
}

func convertLocal(sources map[string][]byte, scopes *scopes,
	local *configs.Local) (hclwrite.Tokens, string, hclwrite.Tokens, hclwrite.Tokens) {
	identifier := scopes.roots["local."+local.Name]
	expr := convertExpression(sources, scopes, "", local.Expr)
	leading, trailing := getTrivia(sources, local.DeclRange)
	return leading, identifier, expr, trailing
}

func convertDataResource(sources map[string][]byte,
	info il.ProviderInfoSource, scopes *scopes,
	dataResource *configs.Resource,
) (hclwrite.Tokens, string, hclwrite.Tokens, hclwrite.Tokens) {
	// We translate dataResources into invokes
	pulumiName := scopes.roots["data."+dataResource.Type+"."+dataResource.Name]

	invokeToken := cty.StringVal(impliedToken(dataResource.Type))
	provider := impliedProvider(dataResource.Type)
	providerInfo, err := info.GetProviderInfo("", "", provider, "")
	if err == nil {
		dataResourceInfo := providerInfo.DataSources[dataResource.Type]
		if dataResourceInfo != nil {
			invokeToken = cty.StringVal(dataResourceInfo.Tok.String())
		}
	}

	// If count is set we'll make this into an array expression
	var countExpr hclwrite.Tokens
	if dataResource.Count != nil {
		countExpr = convertExpression(sources, scopes, "", dataResource.Count)
		scopes.countIndex = hcl.Traversal{hcl.TraverseRoot{Name: "__index"}}
	}

	// If for_each is set we'll make this into an object expression
	var forEachExpr hclwrite.Tokens
	if dataResource.ForEach != nil {
		forEachExpr = convertExpression(sources, scopes, "", dataResource.ForEach)
		scopes.eachKey = hcl.Traversal{hcl.TraverseRoot{Name: "__key"}}
		scopes.eachValue = hcl.Traversal{hcl.TraverseRoot{Name: "__value"}}
	}

	invokeArgs := convertBody(sources, scopes, "data."+dataResource.Type, dataResource.Config)

	functionCall := hclwrite.TokensForFunctionCall(
		"invoke",
		hclwrite.TokensForValue(invokeToken),
		tokensForObject(invokeArgs))

	dataResourceExpression := functionCall
	// If count is set then we need to turn this into a for array expression
	if dataResource.Count != nil {
		dataResourceExpression = hclwrite.Tokens{makeToken(hclsyntax.TokenOBrack, "[")}
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "for"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "__index"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "in"))
		dataResourceExpression = append(dataResourceExpression, hclwrite.TokensForFunctionCall("range", countExpr)...)
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenColon, ":"))
		dataResourceExpression = append(dataResourceExpression, functionCall...)
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenCBrack, "]"))
	}

	// If for_each is set then we need to turn this into a for object expression
	if dataResource.ForEach != nil {
		dataResourceExpression = hclwrite.Tokens{makeToken(hclsyntax.TokenOBrace, "{")}
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "for"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "__key"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenComma, ","))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "__value"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "in"))
		dataResourceExpression = append(dataResourceExpression, forEachExpr...)
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenColon, ":"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenIdent, "__key"))
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenFatArrow, "=>"))
		dataResourceExpression = append(dataResourceExpression, functionCall...)
		dataResourceExpression = append(dataResourceExpression, makeToken(hclsyntax.TokenCBrace, "}"))
	}

	// Clear any count and each
	scopes.countIndex = nil
	scopes.eachKey = nil
	scopes.eachValue = nil
	leading, trailing := getTrivia(sources, dataResource.DeclRange)
	return leading, pulumiName, dataResourceExpression, trailing
}

func convertManagedResources(sources map[string][]byte,
	info il.ProviderInfoSource, scopes *scopes,
	managedResource *configs.Resource,
) (hclwrite.Tokens, *hclwrite.Block, hclwrite.Tokens) {
	// We translate managedResources into resources
	pulumiName := scopes.roots[managedResource.Type+"."+managedResource.Name]

	resourceToken := impliedToken(managedResource.Type)
	provider := impliedProvider(managedResource.Type)
	providerInfo, err := info.GetProviderInfo("", "", provider, "")
	if err == nil {
		managedResourceInfo := providerInfo.Resources[managedResource.Type]
		if managedResourceInfo != nil {
			resourceToken = managedResourceInfo.Tok.String()
		}
	}

	labels := []string{pulumiName, resourceToken}
	block := hclwrite.NewBlock("resource", labels)
	blockBody := block.Body()

	// Does this resource have a count? If so set the "range" attribute
	if managedResource.Count != nil {
		options := blockBody.AppendNewBlock("options", nil)
		countExpr := convertExpression(sources, scopes, "", managedResource.Count)
		// Set the count_index scope
		scopes.countIndex = hcl.Traversal{hcl.TraverseRoot{Name: "range"}, hcl.TraverseAttr{Name: "value"}}
		options.Body().SetAttributeRaw("range", countExpr)
	}
	if managedResource.ForEach != nil {
		options := blockBody.AppendNewBlock("options", nil)
		forEachExpr := convertExpression(sources, scopes, "", managedResource.ForEach)
		scopes.eachKey = hcl.Traversal{hcl.TraverseRoot{Name: "range"}, hcl.TraverseAttr{Name: "key"}}
		scopes.eachValue = hcl.Traversal{hcl.TraverseRoot{Name: "range"}, hcl.TraverseAttr{Name: "value"}}
		options.Body().SetAttributeRaw("range", forEachExpr)
	}

	resourceArgs := convertBody(sources, scopes, managedResource.Type, managedResource.Config)
	for _, arg := range resourceArgs {
		blockBody.SetAttributeRaw(arg.Name, arg.Value)
	}

	// Clear any index we set
	scopes.countIndex = nil
	scopes.eachKey = nil
	scopes.eachValue = nil
	leading, trailing := getTrivia(sources, managedResource.DeclRange)
	return leading, block, trailing
}

func convertModuleCall(
	sources map[string][]byte,
	scopes *scopes,
	moduleCall *configs.ModuleCall) (hclwrite.Tokens, *hclwrite.Block, hclwrite.Tokens) {
	// We translate managedResources into resources
	pulumiName := scopes.roots["module."+moduleCall.Name]
	labels := []string{pulumiName, moduleCall.SourceAddrRaw}
	block := hclwrite.NewBlock("component", labels)
	blockBody := block.Body()

	// Does this resource have a count? If so set the "range" attribute
	if moduleCall.Count != nil {
		options := blockBody.AppendNewBlock("options", nil)
		countExpr := convertExpression(sources, scopes, "", moduleCall.Count)
		// Set the count_index scope
		scopes.countIndex = hcl.Traversal{hcl.TraverseRoot{Name: "range"}, hcl.TraverseAttr{Name: "value"}}
		options.Body().SetAttributeRaw("range", countExpr)
	}

	if moduleCall.ForEach != nil {
		options := blockBody.AppendNewBlock("options", nil)
		forEachExpr := convertExpression(sources, scopes, "", moduleCall.ForEach)
		scopes.eachKey = hcl.Traversal{hcl.TraverseRoot{Name: "range"}, hcl.TraverseAttr{Name: "key"}}
		scopes.eachValue = hcl.Traversal{hcl.TraverseRoot{Name: "range"}, hcl.TraverseAttr{Name: "value"}}
		options.Body().SetAttributeRaw("range", forEachExpr)
	}

	moduleArgs := convertBody(sources, scopes, pulumiName, moduleCall.Config)
	for _, arg := range moduleArgs {
		blockBody.SetAttributeRaw(arg.Name, arg.Value)
	}

	// Clear any index we set
	scopes.countIndex = nil
	scopes.eachKey = nil
	scopes.eachValue = nil
	leading, trailing := getTrivia(sources, moduleCall.DeclRange)
	return leading, block, trailing
}

func convertOutput(sources map[string][]byte, scopes *scopes,
	output *configs.Output) (hclwrite.Tokens, *hclwrite.Block, hclwrite.Tokens) {
	labels := []string{scopes.roots["output."+output.Name]}
	block := hclwrite.NewBlock("output", labels)
	blockBody := block.Body()
	leading, _ := getTrivia(sources, getAttributeRange(sources, output.Expr.Range()))
	blockBody.AppendUnstructuredTokens(leading)
	blockBody.SetAttributeRaw("value", convertExpression(sources, scopes, "", output.Expr))

	leading, trailing := getTrivia(sources, output.DeclRange)
	return leading, block, trailing
}

type scopes struct {
	info il.ProviderInfoSource

	roots map[string]string
	// Local variables that are in scope from for expressions
	locals []map[string]string
	// Set non-nil if "count.index" can be mapped
	countIndex hcl.Traversal
	eachKey    hcl.Traversal
	eachValue  hcl.Traversal
}

// lookup the given name in roots and locals
func (s *scopes) lookup(name string) string {
	for i := len(s.locals) - 1; i >= 0; i-- {
		if s.locals[i][name] != "" {
			return s.locals[i][name]
		}
	}
	if s.roots[name] != "" {
		return s.roots[name]
	}
	return ""
}

func (s *scopes) push(locals map[string]string) {
	s.locals = append(s.locals, locals)
}

func (s *scopes) pop() {
	s.locals = s.locals[0 : len(s.locals)-1]
}

// isUsed returns if _any_ root scope currently uses the name "name"
func (s *scopes) isUsed(name string) bool {
	// We don't have many, but there's a few _keywords_ in pcl that are easier if we just never emit them
	if name == "range" {
		return true
	}

	for _, usedName := range s.roots {
		if usedName == name {
			return true
		}
	}
	return false
}

// generateUniqueName takes "name" and ensures it's unique.
// First by appending `suffix` to it, and then appending an incrementing count
func (s *scopes) generateUniqueName(name, prefix, suffix string) string {
	// Not used, just return it
	if !s.isUsed(name) {
		return name
	}
	// It's used, so add the prefix and suffix
	name = prefix + name + suffix
	if !s.isUsed(name) {
		return name
	}
	// Still used add a counter
	baseName := name
	counter := 2
	for {
		name = fmt.Sprintf("%s%d", baseName, counter)
		if !s.isUsed(name) {
			return name
		}
		counter = counter + 1
	}
}

// getPulumiName takes "name" and ensures it's unique.
// First by appending `suffix` to it, and then appending an incrementing count
func (s *scopes) getOrAddPulumiName(name, prefix, suffix string) string {
	pulumiName := s.roots[name]
	if pulumiName != "" {
		return pulumiName
	}
	parts := strings.Split(name, ".")
	tfName := parts[len(parts)-1]
	pulumiName = camelCaseName(tfName)
	pulumiName = s.generateUniqueName(pulumiName, prefix, suffix)
	s.roots[name] = pulumiName
	return pulumiName
}

// Given a fully qualified and absolute terraform path (e.g. data.simple_data_source.a_field) returns the
// terraform schema for that variable This will return either a shim.Resource or shim.Schema (Wouldn't DUs be
// nice)
func (s *scopes) getTerraformSchema(fullyQualifiedPath string) interface{} {
	parts := strings.Split(fullyQualifiedPath, ".")
	contract.Assertf(len(parts) != 0, "empty path passed into pulumiName")
	var getInner func(sch shim.SchemaMap, parts []string) shim.Schema
	getInner = func(sch shim.SchemaMap, parts []string) shim.Schema {
		// Lookup the info for each part
		if sch == nil {
			return nil
		}

		curSch := sch.Get(parts[0])
		// We want the schema of this part
		if len(parts) == 1 {
			return curSch
		}
		// Else recurse into the next part of the type
		var nextSchema shim.SchemaMap
		if curSch != nil {
			if sch, ok := curSch.Elem().(shim.Resource); ok {
				nextSchema = sch.Schema()
			}
		}
		return getInner(nextSchema, parts[1:])
	}

	if parts[0] == "data" {
		typ := parts[1]
		contract.Assertf(typ != "", "empty data type passed into pulumiName")

		provider := impliedProvider(typ)
		providerInfo, err := s.info.GetProviderInfo("", "", provider, "")
		var currentSchema shim.SchemaMap
		if err == nil {
			sch := providerInfo.P.DataSourcesMap().Get(typ)
			if sch != nil {
				currentSchema = sch.Schema()
			}
		}

		if len(parts) == 2 {
			return currentSchema
		}
		return getInner(currentSchema, parts[2:])
	}

	// This is only for looking up types so this must be a resource
	typ := parts[0]

	provider := impliedProvider(typ)
	providerInfo, err := s.info.GetProviderInfo("", "", provider, "")
	var currentSchema shim.SchemaMap
	if err == nil {
		sch := providerInfo.P.ResourcesMap().Get(typ)
		if sch != nil {
			currentSchema = sch.Schema()
		}
	}

	if len(parts) == 1 {
		return currentSchema
	}
	return getInner(currentSchema, parts[1:])
}

// Given a fully typed path (e.g. data.simple_data_source.a_field) returns the pulumi name for that variable
func (s *scopes) pulumiName(fullyQualifiedPath string) string {
	parts := strings.Split(fullyQualifiedPath, ".")
	contract.Assertf(len(parts) != 0, "empty path passed into pulumiName")
	contract.Assertf(parts[0] != "", "empty path part passed into pulumiName")

	var getInner func(sch shim.SchemaMap, info map[string]*tfbridge.SchemaInfo, parts []string) string
	getInner = func(sch shim.SchemaMap, info map[string]*tfbridge.SchemaInfo, parts []string) string {
		contract.Assertf(parts[0] != "", "empty path part passed into pulumiName")

		// Lookup the info for this part
		var curSch shim.Schema
		if sch != nil {
			curSch = sch.Get(parts[0])
		}
		curInfo := info[parts[0]]

		// We want the name of this part
		if len(parts) == 1 {
			if curInfo != nil && curInfo.Name != "" {
				return curInfo.Name
			}
			return tfbridge.TerraformToPulumiNameV2(parts[0],
				schema.SchemaMap(map[string]shim.Schema{parts[0]: curSch}),
				map[string]*tfbridge.SchemaInfo{parts[0]: curInfo})
		}
		// Else recurse into the next part of the type
		var nextSchema shim.SchemaMap
		var nextInfo map[string]*tfbridge.SchemaInfo
		if curSch != nil {
			if sch, ok := curSch.Elem().(shim.Resource); ok {
				nextSchema = sch.Schema()
			}
		}
		if curInfo != nil {
			nextInfo = curInfo.Fields
		}
		return getInner(nextSchema, nextInfo, parts[1:])
	}

	if parts[0] == "data" {
		typ := parts[1]
		contract.Assertf(typ != "", "empty data type passed into pulumiName")

		provider := impliedProvider(typ)
		providerInfo, err := s.info.GetProviderInfo("", "", provider, "")
		var currentSchema shim.SchemaMap
		var currentInfo map[string]*tfbridge.SchemaInfo
		if err == nil {
			sch := providerInfo.P.DataSourcesMap().Get(typ)
			if sch != nil {
				currentSchema = sch.Schema()
			}
			if dataSource, has := providerInfo.DataSources[typ]; has {
				currentInfo = dataSource.Fields
			}
		}

		if len(parts) == 2 {
			return camelCaseName(typ)
		}
		return getInner(currentSchema, currentInfo, parts[2:])
	}

	// This is only for looking up types so this must be a resource
	typ := parts[0]

	provider := impliedProvider(typ)
	providerInfo, err := s.info.GetProviderInfo("", "", provider, "")
	var currentSchema shim.SchemaMap
	var currentInfo map[string]*tfbridge.SchemaInfo
	if err == nil {
		sch := providerInfo.P.ResourcesMap().Get(typ)
		if sch != nil {
			currentSchema = sch.Schema()
		}
		if resource, has := providerInfo.Resources[typ]; has {
			currentInfo = resource.Fields
		}
	}

	if len(parts) == 1 {
		return camelCaseName(typ)
	}
	return getInner(currentSchema, currentInfo, parts[1:])
}

// An "item" from a terraform file
type terraformItem struct {
	variable   *configs.Variable
	local      *configs.Local
	data       *configs.Resource
	moduleCall *configs.ModuleCall
	resource   *configs.Resource
	output     *configs.Output
}

func (item terraformItem) DeclRange() hcl.Range {
	if item.variable != nil {
		return item.variable.DeclRange
	}
	if item.local != nil {
		return item.local.DeclRange
	}
	if item.data != nil {
		return item.data.DeclRange
	}
	if item.resource != nil {
		return item.resource.DeclRange
	}
	if item.moduleCall != nil {
		return item.moduleCall.DeclRange
	}
	if item.output != nil {
		return item.output.DeclRange
	}
	panic("at least one of the fields in terraformItem should be set!")
}

type terraformItems []terraformItem

func (items terraformItems) Len() int      { return len(items) }
func (items terraformItems) Swap(i, j int) { items[i], items[j] = items[j], items[i] }
func (items terraformItems) Less(i, j int) bool {
	a := items[i].DeclRange()
	b := items[j].DeclRange()

	if a.Filename < b.Filename {
		return true
	} else if a.Filename > b.Filename {
		return false
	} else {
		return a.Start.Line < b.Start.Line
	}
}

func translateModuleInternal(source afero.Fs,
	destination afero.Fs, info il.ProviderInfoSource, directory string) hcl.Diagnostics {

	sources, module, moduleDiagnostics := loadConfigDir(source, directory)
	if moduleDiagnostics.HasErrors() {
		// No syntax.Files to return here because we're relying on terraform to load and parse, means no
		// source context gets printed with warnings/errors here.
		return moduleDiagnostics
	}

	scopes := &scopes{
		info:   info,
		roots:  make(map[string]string),
		locals: make([]map[string]string, 0),
	}

	// First go through and add everything to the items list so we can sort it by source order
	items := make(terraformItems, 0)
	for _, variable := range module.Variables {
		items = append(items, terraformItem{variable: variable})
	}
	for _, local := range module.Locals {
		items = append(items, terraformItem{local: local})
	}
	for _, data := range module.DataResources {
		items = append(items, terraformItem{data: data})
	}
	for _, moduleCall := range module.ModuleCalls {
		items = append(items, terraformItem{moduleCall: moduleCall})
	}
	for _, resource := range module.ManagedResources {
		items = append(items, terraformItem{resource: resource})
	}
	for _, output := range module.Outputs {
		items = append(items, terraformItem{output: output})
	}
	// Now sort that items array by source location
	sort.Sort(items)

	// Now go through and generate unique names for all the things
	for _, item := range items {
		if item.variable != nil {
			scopes.getOrAddPulumiName("var."+item.variable.Name, "", "Config")
		}
	}
	for _, item := range items {
		if item.local != nil {
			scopes.getOrAddPulumiName("local."+item.local.Name, "my", "")
		}
	}
	for _, item := range items {
		if item.data != nil {
			dataResource := item.data
			scopes.getOrAddPulumiName("data."+dataResource.Type+"."+dataResource.Name, "", "Data")
		}
	}
	for _, item := range items {
		if item.resource != nil {
			managedResource := item.resource
			scopes.getOrAddPulumiName(managedResource.Type+"."+managedResource.Name, "", "Resource")
		}
	}
	for _, item := range items {
		if item.moduleCall != nil {
			moduleCall := item.moduleCall
			scopes.getOrAddPulumiName("module."+moduleCall.Name, "", "Component")
			modulePath := filepath.Join(directory, moduleCall.SourceAddrRaw)
			moduleDiags := translateModuleInternal(source, destination, info, modulePath)
			if moduleDiags.HasErrors() {
				return moduleDiags
			}
		}
	}

	for _, item := range items {
		if item.output != nil {
			scopes.getOrAddPulumiName("output."+item.output.Name, "", "Output")
		}
	}

	pclFiles := make(map[string]*hclwrite.File)

	// We want to write things out to matching .pp files and in source order
	for _, item := range items {
		r := item.DeclRange()
		path := changeExtension(r.Filename, ".pp")
		path, err := filepath.Rel(directory, path)
		if err != nil {
			panic("Rel should never fail")
		}
		file := pclFiles[path]
		if file == nil {
			file = hclwrite.NewFile()
			pclFiles[path] = file
		}

		body := file.Body()

		// First handle any inputs, these will be picked up by the "vars" scope
		if item.variable != nil {
			leading, block, trailing := convertVariable(sources, scopes, item.variable)
			body.AppendUnstructuredTokens(leading)
			body.AppendBlock(block)
			body.AppendUnstructuredTokens(trailing)
		}
		// Next handle any locals, these will be picked up by the "locals" scope
		if item.local != nil {
			leading, name, value, trailing := convertLocal(sources, scopes, item.local)
			body.AppendUnstructuredTokens(leading)
			body.SetAttributeRaw(name, value)
			body.AppendUnstructuredTokens(trailing)
		}
		// Next handle any data sources
		if item.data != nil {
			leading, name, value, trailing := convertDataResource(sources, info, scopes, item.data)
			body.AppendUnstructuredTokens(leading)
			body.SetAttributeRaw(name, value)
			body.AppendUnstructuredTokens(trailing)
		}
		// Next handle any resources
		if item.resource != nil {
			leading, block, trailing := convertManagedResources(sources, info, scopes, item.resource)
			body.AppendUnstructuredTokens(leading)
			body.AppendBlock(block)
			body.AppendUnstructuredTokens(trailing)
		}
		// Next handle any modules
		if item.moduleCall != nil {
			leading, block, trailing := convertModuleCall(sources, scopes, item.moduleCall)
			body.AppendUnstructuredTokens(leading)
			body.AppendBlock(block)
			body.AppendUnstructuredTokens(trailing)
		}
		// Finally handle any outputs
		if item.output != nil {
			leading, block, trailing := convertOutput(sources, scopes, item.output)
			body.AppendUnstructuredTokens(leading)
			body.AppendBlock(block)
			body.AppendUnstructuredTokens(trailing)
		}
	}

	// Now we've written everything generate formatted output files
	for key, file := range pclFiles {
		buffer := &bytes.Buffer{}
		_, err := file.WriteTo(buffer)
		if err != nil {
			return hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("could not write pcl to memory buffer: %s", err),
			}}
		}

		fullpath := filepath.Join(directory, key)
		keyDirectory := filepath.Dir(fullpath)
		err = destination.MkdirAll(keyDirectory, 0755)
		if err != nil {
			return hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("could not create destination directory for pcl: %s", err),
			}}
		}

		// Reformat to canonical style
		formatted := hclwrite.Format(buffer.Bytes())
		err = afero.WriteFile(destination, fullpath, formatted, 0644)
		if err != nil {
			return hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("could not write pcl to destination: %s", err),
			}}
		}
	}
	return nil
}

func TranslateModule(source afero.Fs, destination afero.Fs, info il.ProviderInfoSource) hcl.Diagnostics {
	return translateModuleInternal(source, destination, info, "/")
}

func errorf(subject hcl.Range, f string, args ...interface{}) *hcl.Diagnostic {
	return diagf(hcl.DiagError, subject, f, args...)
}

func diagf(severity hcl.DiagnosticSeverity, subject hcl.Range, f string, args ...interface{}) *hcl.Diagnostic {
	message := fmt.Sprintf(f, args...)
	return &hcl.Diagnostic{
		Severity: severity,
		Summary:  message,
		Subject:  &subject,
	}
}

func componentProgramBinderFromAfero(fs afero.Fs) pcl.ComponentProgramBinder {
	return func(args pcl.ComponentProgramBinderArgs) (*pcl.Program, hcl.Diagnostics, error) {
		var diagnostics hcl.Diagnostics
		binderDirPath := args.BinderDirPath
		componentSource := args.ComponentSource
		nodeRange := args.ComponentNodeRange
		loader := args.BinderLoader
		// bind the component here as if it was a new program
		// this becomes the DirPath for the new binder
		componentSourceDir := filepath.Join(binderDirPath, componentSource)

		parser := syntax.NewParser()
		// Load all .pp files in the components' directory
		files, err := afero.ReadDir(fs, componentSourceDir)
		if err != nil {
			diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
			return nil, diagnostics, nil
		}

		if len(files) == 0 {
			diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
			return nil, diagnostics, nil
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			fileName := file.Name()
			path := filepath.Join(componentSourceDir, fileName)

			if filepath.Ext(fileName) == ".pp" {
				file, err := fs.Open(path)
				if err != nil {
					diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
					return nil, diagnostics, err
				}

				err = parser.ParseFile(file, fileName)

				if err != nil {
					diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
					return nil, diagnostics, err
				}

				diags := parser.Diagnostics
				if diags.HasErrors() {
					return nil, diagnostics, err
				}
			}
		}

		if err != nil {
			diagnostics = diagnostics.Append(errorf(nodeRange, err.Error()))
			return nil, diagnostics, err
		}

		componentProgram, programDiags, err := pcl.BindProgram(parser.Files,
			pcl.Loader(loader),
			pcl.DirPath(componentSourceDir),
			pcl.ComponentBinder(componentProgramBinderFromAfero(fs)))

		return componentProgram, programDiags, err
	}
}

func convertTerraform(opts EjectOptions) ([]*syntax.File, *pcl.Program, hcl.Diagnostics, error) {
	var pulumiOptions []pcl.BindOption
	if opts.AllowMissingProperties {
		pulumiOptions = append(pulumiOptions, pcl.AllowMissingProperties)
	}
	if opts.AllowMissingVariables {
		pulumiOptions = append(pulumiOptions, pcl.AllowMissingVariables)
	}
	// PluginHost just sets loader internally
	if opts.PluginHost != nil && opts.Loader == nil {
		pulumiOptions = append(pulumiOptions, pcl.PluginHost(opts.PluginHost))
	}
	if opts.Loader != nil {
		pulumiOptions = append(pulumiOptions, pcl.Loader(opts.Loader))
	}
	if opts.PackageCache != nil {
		pulumiOptions = append(pulumiOptions, pcl.Cache(opts.PackageCache))
	}
	if opts.SkipResourceTypechecking {
		pulumiOptions = append(pulumiOptions, pcl.SkipResourceTypechecking)
	}

	rootDir := "/"
	tempDir := afero.NewMemMapFs()

	diagnostics := TranslateModule(opts.Root, tempDir, opts.ProviderInfoSource)
	if diagnostics.HasErrors() {
		return nil, nil, diagnostics, diagnostics
	}

	pulumiOptions = append(pulumiOptions, pcl.DirPath(rootDir))
	pulumiOptions = append(pulumiOptions, pcl.ComponentBinder(componentProgramBinderFromAfero(tempDir)))

	pulumiParser := syntax.NewParser()

	files, err := afero.ReadDir(tempDir, rootDir)
	if err != nil {
		return nil, nil, diagnostics, fmt.Errorf("could not read files at the root: %v", err)
	}

	for _, file := range files {
		fileName := file.Name()
		path := filepath.Join(rootDir, fileName)
		if filepath.Ext(path) == ".pp" {
			reader, err := tempDir.Open(path)
			if err != nil {
				return nil, nil, diagnostics, err
			}
			contract.AssertNoErrorf(err, "reading file should work")
			err = pulumiParser.ParseFile(reader, filepath.Base(path))
			contract.AssertNoErrorf(err, "parsing file should work")
			if pulumiParser.Diagnostics.HasErrors() {
				file, err := afero.ReadFile(tempDir, path)
				contract.AssertNoErrorf(err, "reading file should work")
				opts.logf("%s", string(file))
				opts.logf("%v", pulumiParser.Diagnostics)
				return nil, nil, pulumiParser.Diagnostics, pulumiParser.Diagnostics
			}
		}
	}

	if err != nil {
		return pulumiParser.Files, nil, pulumiParser.Diagnostics, err
	}

	program, diagnostics, err := pcl.BindProgram(pulumiParser.Files, pulumiOptions...)

	return pulumiParser.Files, program, diagnostics, err
}
