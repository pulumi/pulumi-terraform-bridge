package schemafilter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func FilterSchemaByLanguage(schemaBytes []byte, language string) []byte {
	// The span string stems from the Terraform bridge's generator's fixUpPropertyReference method in docsgen.
	// It looks as follows:
	// <span pulumi-lang-nodejs="firstProperty" pulumi-lang-go="FirstProperty" ...>first_property</span>
	// When rendered in schema it uses escapes and unicode chars for the angle brackets:
	// \u003cspan pulumi-lang-nodejs=\"`random.RandomBytes`\" pulumi-lang-dotnet=\"`random.RandomBytes`\" ... \u003e ...
	spanRegex := regexp.MustCompile(`\\u003cspan pulumi-lang-nodejs=.*?\\u003c/span\\u003e`)

	// Extract the language-specific inflection for the found inflection span
	schemaBytes = spanRegex.ReplaceAllFunc(schemaBytes, func(match []byte) []byte {
		languageKey := []byte(fmt.Sprintf(`pulumi-lang-%s=\"`, language))
		_, startLanguageValue, _ := bytes.Cut(match, languageKey)
		var languageValue []byte

		// Sometimes we have double quotes in our language span. Handle this case so that we return the quotes.
		doubleEscapedQuotes := []byte(`\"\"`)
		singleEscapedQuotes := []byte(`\"`)
		if loc := bytes.Index(startLanguageValue, doubleEscapedQuotes); loc > 0 {
			// Cut after the first quote to include it in the result
			languageValue = startLanguageValue[:loc+(len(singleEscapedQuotes))]
		} else {
			languageValue, _, _ = bytes.Cut(startLanguageValue, singleEscapedQuotes)
		}
		return languageValue
	})

	// Find code chooser blocks and filter to only keep the current language
	codeChooserRegex := regexp.MustCompile(
		`\\u003c!--Start PulumiCodeChooser --\\u003e.*?\\u003c!--End PulumiCodeChooser --\\u003e`,
	)

	schemaBytes = codeChooserRegex.ReplaceAllFunc(schemaBytes, func(match []byte) []byte {
		content := string(match)

		// In code choosers for registry docsgen, "nodejs" is "typescript"
		codeLang := language
		if language == "nodejs" {
			codeLang = "typescript"
		}
		// In code choosers, "dotnet" is "csharp"
		if language == "dotnet" {
			codeLang = "csharp"
		}
		// Extract language-specific example only
		_, after, found := strings.Cut(content, fmt.Sprintf("```%s", codeLang))
		if !found {
			return []byte("")
		}
		codeForLanguage, _, found := strings.Cut(after, "```")
		if !found {
			return []byte("")
		}
		codeForLanguage = fmt.Sprintf("```%s", codeLang) + codeForLanguage + "```"

		return []byte(codeForLanguage)
	})
	return schemaBytes
}
