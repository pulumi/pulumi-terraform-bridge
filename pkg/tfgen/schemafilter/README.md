# provider-schema-filter
A small library for provider schema processing

> [NOTE] This library is meant for Pulumi-internal use. It is experimental and subject to change.

## Purpose

Prepare Pulumi provider schemas to be passed to the pulumi package gen-sdk command.

Pulumi provider schemas (traditionally located at `provider/cmd/pulumi-resource-foo/schema.json`) are our source for generating registry documentation, provider binaries, and language SDKs, including in-line documentation with examples.
The schema contains language-specific translations of examples, as well as language-specific inflections of code strings.

Because the schema contains translations for _all_ Pulumi-supported languages, we need to filter the schema by its language before we pass it along to the Pulumi CLI's SDK generator.
This library is meant for that purpose.
The filter expects a certain schema format, which is consistent for all bridged providers, detailed below.

## Schema markups

The Pulumi schema may contain the following markups:

### Pulumi Code Chooser

The content between the code chooser tags contains the examples that we are rendering in the registry via the language tab selector.
The outline is as follows:
`<!--Start PulumiCodeChooser -->```typescript {example code}```\n```python {example code}```\n```csharp {example code}```\n```go {example code}```/n```java {example code}```\n```yaml {example code}```\n<!--End PulumiCodeChooser -->`
For each SDK, we want to display only the example relevant to that SDK's language.

### Variable inflection

The documentation contains property or resource names that should be inflected by language.
The precise inflection format depends on whether we are inflecting a resource, a function, or a property name, but the basic format loks like this:
`<span pulumi-lang-nodejs="exampleProperty" pulumi-lang-dotnet="ExampleProperty" pulumi-lang-go="exampleProperty" pulumi-lang-python="example_property" pulumi-lang-yaml="exampleProperty" pulumi-lang-java="exampleProperty">%s</span>`

## Native providers

Native providers do not use the outlined markups and so this filter would return the schema as-is. 