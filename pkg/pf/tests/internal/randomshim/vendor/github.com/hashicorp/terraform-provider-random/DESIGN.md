# Random Provider Design

The Random Provider offers focussed functionality specifically geared towards the generation of random values for use
within Terraform configurations. Specifically, the provider resources generate random values at the time they are 
created and then maintain these values unless the resource inputs are altered.

Below we have a collection of _Goals_ and _Patterns_: they represent the guiding principles applied during the
development of this provider. Some are in place, others are ongoing processes, others are still just inspirational.

## Goals

* [_Stability over features_](.github/CONTRIBUTING.md)
* Provide managed resources to generate random values for use within Terraform configurations.
* Support the underlying use of a cryptographic random number generator for
  [id](docs/resources/id.md), [password](docs/resources/id.md) and
  [string](docs/resources/string.md).
* Support the use of "[keepers](docs/index.md)" for all resources.
* Support the use of encoding [id](docs/resources/id.md) as: 
  * [base64](https://www.rfc-editor.org/rfc/rfc4648.html#section-4)
  * [base64 URL](https://www.rfc-editor.org/rfc/rfc4648.html#section-5)
  * [decimal](https://pkg.go.dev/math/big#Int.String)
  * [hex](https://pkg.go.dev/math/big#Int.String)

## Patterns

Specific to this provider:

* The generation of [password](docs/resources/password.md) and
  [string](docs/resources/string.md) use exactly the same underlying code, the only 
  difference is that the output from *password* is treated as 
  [sensitive](https://www.terraform.io/language/state/sensitive-data).

General to development:

* **Avoid repetition**: the entities managed can sometimes require similar pieces of logic and/or schema to be realised.
  When this happens it's important to keep the code shared in communal sections, so to avoid having to modify code in
  multiple places when they start changing.
* **Test expectations as well as bugs**: While it's typical to write tests to exercise a new functionality, it's key to
  also provide tests for issues that get identified and fixed, so to prove resolution as well as avoid regression.
* **Automate boring tasks**: Processes that are manual, repetitive and can be automated, should be. In addition to be a
  time-saving practice, this ensures consistency and reduces human error (ex. static code analysis).
* **Semantic versioning**: Adhering to HashiCorp's own
  [Versioning Specification](https://www.terraform.io/plugin/sdkv2/best-practices/versioning#versioning-specification)
  ensures we provide a consistent practitioner experience, and a clear process to deprecation and decommission.
