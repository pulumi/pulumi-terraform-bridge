// Package unrec implements recursion detection over [Pulumi Package Schema].
//
// The implementation is generic to all providers but the intent is tailored for bridged providers specifically. TF
// cannot represent recursive types but uses unrolling up to level N, for example for Statement in waf2 web_acl in AWS
// it uses unrolling to level 3. This creates too many types in the Pulumi projection.
//
// This package is aimed at detecting the unrolled recursion and tying it back into recursive types.
//
// [Pulumi Package Schema]: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
package unrec
