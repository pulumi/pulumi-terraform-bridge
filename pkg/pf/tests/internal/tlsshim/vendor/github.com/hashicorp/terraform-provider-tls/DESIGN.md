# TLS Provider Design

The TLS Provider offers a small surface area compared to other providers (like
[AWS](https://registry.terraform.io/providers/hashicorp/aws/latest),
[Google](https://registry.terraform.io/providers/hashicorp/google/latest),
[Azure](https://registry.terraform.io/providers/hashicorp/azurerm/latest), ...),
and focuses on covering the needs of working with entities like
keys and certificates, that are part of
[Transport Layer Security](https://en.wikipedia.org/wiki/Transport_Layer_Security).

Below we have a collection of _Goals_ and _Patterns_: they represent the guiding principles applied during
the development of this provider. Some are in place, others are ongoing processes, others are still just inspirational.
 
## Goals

* [_Stability over features_](.github/CONTRIBUTING.md) 
* Support [cryptography](https://en.wikipedia.org/wiki/Cryptography) _primitives_ necessary to Terraform configurations
* Provide managed resourced and data sources to manipulate and interact with **Keys, Certificates and Certificate Requests**
* Support formats, backed by [IETF RFCs](https://www.ietf.org/standards/rfcs/):
  * [Privacy Enhancement for Internet Electronic Mail (PEM) (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421)
  * [Internet X.509 Public Key Infrastructure Certificate (RFC 5280)](https://datatracker.ietf.org/doc/html/rfc5280)
  * [Secure Shell (SSH) Public Key Format (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716),
    as well as [SSH Private Key format](https://coolaj86.com/articles/the-openssh-private-key-format/)
  * [Public-Key Cryptography Standards (PKCS) #8 (RFC 5208)](https://datatracker.ietf.org/doc/html/rfc5208)
  * [Distinguished Names representation (RFC 2253)](https://datatracker.ietf.org/doc/html/rfc2253)
  * [Timestamps (RFC 3339)](https://datatracker.ietf.org/doc/html/rfc3339)
* Support specific cryptography key algorithms:
  * [`RSA`](https://en.wikipedia.org/wiki/RSA_(cryptosystem))
  * [`ECDSA`](https://en.wikipedia.org/wiki/Elliptic_Curve_Digital_Signature_Algorithm)
    with curves `P224`, `P256`, `P384` and `P521`
  * [`ED25519`](https://ed25519.cr.yp.to/)
* For implementation of cryptographic primitives we will stick with Golang [crypto](https://pkg.go.dev/crypto)
  and [x/crypto](https://pkg.go.dev/golang.org/x/crypto)
  * Cryptography is a non-trivial subject, and not all provider maintainers can also be domain experts
  * We will only support technologies that are covered by these libraries
  * In rare cases we _might_ consider using implementations from other repositories, but they will be
    entirely at the discretion of the maintenance team to judge the quality, maintenance status and community adoption
    of those repositories
* Provide a comprehensive documentation
* Highlight intended and unadvisable usages

### About formats and key algorithms

Cryptography and security are an evolving and changing subject; for this reason the set of technologies supported 
will need to be reassessed over time by the maintenance team,
while also evaluating incoming [feature requests](.github/CONTRIBUTING.md#feature-requests).

## Patterns

Specific to this provider:

* **Consistency**: once a format or algorithm is adopted, all resources and data sources should support it (if appropriate)
* **`PEM` and `OpenSSH PEM`**: Entities that support [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421)
  should also support [OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716), unless there is a good
  reason not to.
* **No ["security by obscurity"](https://en.wikipedia.org/wiki/Security_through_obscurity)**: We should be clear
  in implementation and documentation that this provider doesn't provide "security" per se, but it's up to the
  practitioner to ensure it, by setting in place the right infrastructure, like storing the Terraform state in
  accordance with [recommendations](https://www.terraform.io/language/state/sensitive-data#recommendations).

General to development:

* **Avoid repetition**: the entities managed can sometimes require similar pieces of logic and/or schema to be realised.
  When this happens it's important to keep the code shared in communal sections, so to avoid having to modify code
  in multiple places when they start changing.
* **Test expectations as well as bugs**: While it's typical to write tests to exercise a new functionality, it's key
  to also provide tests for issues that get identified and fixed, so to prove resolution as well as avoid regression.
* **Automate boring tasks**: Processes that are manual, repetitive and can be automated, should be.
  In addition to be a time-saving practice, this ensures consistency and reduces human error (ex. static code analysis).
* **Semantic versioning**: Adhering to HashiCorp's own
  [Versioning Specification](https://www.terraform.io/plugin/sdkv2/best-practices/versioning#versioning-specification)
  ensures we provide a consistent practitioner experience, and a clear process to deprecation and decommission.
