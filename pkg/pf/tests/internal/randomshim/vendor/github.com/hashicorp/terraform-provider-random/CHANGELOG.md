## 3.6.0 (December 04, 2023)

FEATURES:

* resource/random_bytes: New resource that generates an array of random bytes intended to be used as key or secret ([#272](https://github.com/hashicorp/terraform-provider-random/issues/272))

## 3.5.1 (April 12, 2023)

BUG FIXES:

* resource/random_password: Prevent error with `bcrypt` by truncating the bytes that are hashed to a maximum length of 72 ([#397](https://github.com/hashicorp/terraform-provider-random/issues/397))

## 3.5.0 (April 11, 2023)

NOTES:

* This Go module has been updated to Go 1.19 per the [Go support policy](https://golang.org/doc/devel/release.html#policy). Any consumers building on earlier Go versions may experience errors. ([#378](https://github.com/hashicorp/terraform-provider-random/issues/378))

## 3.4.3 (September 08, 2022)

NOTES:

* resource/random_password: The values for `lower`, `number`, `special`, `upper`, `min_lower`, `min_numeric`, `min_special`, `min_upper` and `length` could be null if the resource was imported using version 3.3.1 or before. The value for `length` will be automatically calculated and assigned and default values will be assigned for the other attributes listed after this upgrade ([#313](https://github.com/hashicorp/terraform-provider-random/pull/313))
* resource/random_string: The values for `lower`, `number`, `special`, `upper`, `min_lower`, `min_numeric`, `min_special`, `min_upper` and `length` could be null if the resource was imported using version 3.3.1 or before. The value for `length` will be automatically calculated and assigned and default values will be assigned for the other attributes listed after this upgrade ([#313](https://github.com/hashicorp/terraform-provider-random/pull/313))
* resource/random_password: If the resource was created between versions 3.4.0 and 3.4.2, the `bcrypt_hash` value would not correctly verify against the `result` value. Affected resources will automatically regenerate a valid `bcrypt_hash` after this upgrade. ([#308](https://github.com/hashicorp/terraform-provider-random/pull/308))
* resource/random_password: The `override_special` attribute may show a plan difference from empty string (`""`) to `null` if previously applied with version 3.4.2. The plan should show this as an in-place update and it should occur only once after upgrading. ([#312](https://github.com/hashicorp/terraform-provider-random/pull/312))
* resource/random_string: The `override_special` attribute may show a plan difference from empty string (`""`) to `null` if previously applied with version 3.4.2. The plan should show this as an in-place update and it should occur only once after upgrading. ([#312](https://github.com/hashicorp/terraform-provider-random/pull/312))

BUG FIXES:

* resource/random_password: Assign default values to `lower`, `number`, `special`, `upper`, `min_lower`, `min_numeric`, `min_special` and `min_upper` if null. Assign length of `result` to `length` if null ([#313](https://github.com/hashicorp/terraform-provider-random/pull/313))
* resource/random_string: Assign default values to `lower`, `number`, `special`, `upper`, `min_lower`, `min_numeric`, `min_special` and `min_upper` if null. Assign length of `result` to `length` if null ([#313](https://github.com/hashicorp/terraform-provider-random/pull/313))
* resource/random_password: Fixed incorrect `bcrypt_hash` generation since version 3.4.0 ([#308](https://github.com/hashicorp/terraform-provider-random/pull/308))
* resource/random_password: Prevented difference with `override_special` when upgrading from version 3.3.2 and earlier ([#312](https://github.com/hashicorp/terraform-provider-random/pull/312))
* resource/random_string: Prevented difference with `override_special` when upgrading from version 3.3.2 and earlier ([#312](https://github.com/hashicorp/terraform-provider-random/pull/312))

## 3.4.2 (September 02, 2022)

BUG FIXES:

* all: Prevent `keeper` with `null` values from forcing replacement ([305](https://github.com/hashicorp/terraform-provider-random/pull/305)).
* resource/random_password: During upgrade state, ensure `min_upper` is populated ([304](https://github.com/hashicorp/terraform-provider-random/pull/304)).
* resource/random_string: During upgrade state, ensure `min_upper` is populated ([304](https://github.com/hashicorp/terraform-provider-random/pull/304)).

## 3.4.1 (August 31, 2022)

BUG FIXES:

* resource/random_password: During attribute plan modifier, only return error if `number` and `numeric` are both present and do not match ([301](https://github.com/hashicorp/terraform-provider-random/pull/301)).
* resource/random_string: During attribute plan modifier, only return error if `number` and `numeric` are both present and do not match ([301](https://github.com/hashicorp/terraform-provider-random/pull/301)).

## 3.4.0 (August 30, 2022)

NOTES:

* Provider has been re-written using the new [`terraform-plugin-framework`](https://www.terraform.io/plugin/framework) ([#177](https://github.com/hashicorp/terraform-provider-random/pull/177)).
* resource/random_password: `number` was deprecated in [v3.3.0](https://github.com/hashicorp/terraform-provider-random/releases/tag/v3.3.0) and will be removed in the next major release.
* resource/random_string: `number` was deprecated in [v3.3.0](https://github.com/hashicorp/terraform-provider-random/releases/tag/v3.3.0) and will be removed in the next major release.

## 3.3.2 (June 23, 2022)

BUG FIXES:

* resource/random_password: When importing set defaults for all attributes that have a default defined ([256](https://github.com/hashicorp/terraform-provider-random/pull/256)).
* resource/random_string: When importing set defaults for all attributes that have a default defined ([256](https://github.com/hashicorp/terraform-provider-random/pull/256)).

## 3.3.1 (June 07, 2022)

BUG FIXES:

* resource/random_password: During schema upgrade, copy value of attribute `number` to attribute `numeric`, only if said value is a boolean (i.e. not `null`) ([262](https://github.com/hashicorp/terraform-provider-random/pull/262))
* resource/random_string: During schema upgrade, copy value of attribute `number` to attribute `numeric`, only if said value is a boolean (i.e. not `null`) ([262](https://github.com/hashicorp/terraform-provider-random/pull/262))

## 3.3.0 (June 06, 2022)

ENHANCEMENTS:

* resource/random_password: `number` is now deprecated and `numeric` has been added to align attribute naming. `number` will be removed in the next major release ([#258](https://github.com/hashicorp/terraform-provider-random/pull/258)).
* resource/random_string: `number` is now deprecated and `numeric` has been added to align attribute naming. `number` will be removed in the next major release ([#258](https://github.com/hashicorp/terraform-provider-random/pull/258)).

## 3.2.0 (May 18, 2022)

NEW FEATURES:

* resource/random_password: New attribute `bcrypt_hash`, which is the hashed password ([73](https://github.com/hashicorp/terraform-provider-random/pull/73), [102](https://github.com/hashicorp/terraform-provider-random/issues/102), [254](https://github.com/hashicorp/terraform-provider-random/pull/254))

NOTES:

* Adds or updates DESIGN.md, README.md, CONTRIBUTING.md and SUPPORT.md docs ([176](https://github.com/hashicorp/terraform-provider-random/issues/176), [235](https://github.com/hashicorp/terraform-provider-random/issues/235), [242](https://github.com/hashicorp/terraform-provider-random/pull/242)).
* Removes usage of deprecated fields, types and functions ([243](https://github.com/hashicorp/terraform-provider-random/issues/243), [244](https://github.com/hashicorp/terraform-provider-random/pull/244)).
* Tests all minor Terraform versions ([238](https://github.com/hashicorp/terraform-provider-random/issues/238), [241](https://github.com/hashicorp/terraform-provider-random/pull/241))
* Switches to linting with golangci-lint ([237](https://github.com/hashicorp/terraform-provider-random/issues/237), [240](https://github.com/hashicorp/terraform-provider-random/pull/240)).

## 3.1.3 (April 22, 2022)

BUG FIXES:

* resource/random_password: Prevent crash when length is less than 0 ([#129](https://github.com/hashicorp/terraform-provider-random/issues/129), [#181](https://github.com/hashicorp/terraform-provider-random/issues/181), [#200](https://github.com/hashicorp/terraform-provider-random/pull/200), [#233](https://github.com/hashicorp/terraform-provider-random/issues/233)).
* resource/random_string: Prevent crash when length is less than 0 ([#129](https://github.com/hashicorp/terraform-provider-random/issues/129), [#181](https://github.com/hashicorp/terraform-provider-random/issues/181), [#200](https://github.com/hashicorp/terraform-provider-random/pull/200), [#233](https://github.com/hashicorp/terraform-provider-random/issues/233)).
* resource/random_password: Prevent confusing inconsistent result error when length is 0 ([#222](https://github.com/hashicorp/terraform-provider-random/issues/222), [#233](https://github.com/hashicorp/terraform-provider-random/issues/233)).
* resource/random_string: Prevent confusing inconsistent result error when length is 0 ([#222](https://github.com/hashicorp/terraform-provider-random/issues/222), [#233](https://github.com/hashicorp/terraform-provider-random/issues/233)).

## 3.1.2 (March 17, 2022)

BUG FIXES:

* resource/random_pet: Prevented deterministic results since 3.1.1 ([#217](https://github.com/hashicorp/terraform-provider-random/issues/217). 

## 3.1.1 (March 16, 2022)

NOTES:

* Updated [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs) to `v0.7.0`:
  this improves generated documentation, with attributes now correctly formatted as `code`
  and provided with anchors.
* Functionally identical to the previous 3.1.0 release.

## 3.1.0 (February 19, 2021)

Binary releases of this provider now include the darwin-arm64 platform. This version contains no further changes.

## 3.0.1 (January 12, 2021)

BUG FIXES:

* `resource_integer`: Integers in state that do not cleanly fit into float64s no longer lose their precision ([#132](https://github.com/terraform-providers/terraform-provider-random/issues/132))

## 3.0.0 (October 09, 2020)

Binary releases of this provider will now include the linux-arm64 platform.

BREAKING CHANGES:

* Upgrade to version 2 of the Terraform Plugin SDK, which drops support for Terraform 0.11. This provider will continue to work as expected for users of Terraform 0.11, which will not download the new version. ([#118](https://github.com/terraform-providers/terraform-provider-random/issues/118))
* Remove deprecated `b64` attribute ([#118](https://github.com/terraform-providers/terraform-provider-random/issues/118))

## 2.3.1 (October 26, 2020)

NOTES: This version is identical to v2.3.0, but has been compiled using Go v1.14.5 to fix https://github.com/hashicorp/terraform-provider-random/issues/120.

## 2.3.0 (July 07, 2020)

NOTES:

* The provider now uses the binary driver for acceptance tests ([#99](https://github.com/terraform-providers/terraform-provider-random/issues/99))

NEW FEATURES:

* Added import handling for `random_string` and `random_password` ([#104](https://github.com/terraform-providers/terraform-provider-random/issues/104))

## 2.2.1 (September 25, 2019)

NOTES:

* The provider has switched to the standalone TF SDK, there should be no noticeable impact on compatibility. ([#76](https://github.com/terraform-providers/terraform-provider-random/issues/76))

## 2.2.0 (August 08, 2019)

NEW FEATURES:

* `random_password` is similar to `random_string` but is marked sensitive for logs and output [[#52](https://github.com/terraform-providers/terraform-provider-random/issues/52)] 

## 2.1.2 (April 30, 2019)

* This release includes another Terraform SDK upgrade intended to align with that being used for other providers as we prepare for the Core v0.12.0 release. It should have no significant changes in behavior for this provider.

## 2.1.1 (April 12, 2019)

* This release includes only a Terraform SDK upgrade intended to align with that being used for other providers as we prepare for the Core v0.12.0 release. It should have no significant changes in behavior for this provider.

## 2.1.0 (March 20, 2019)

IMPROVEMENTS:

* The provider is now compatible with Terraform v0.12, while retaining compatibility with prior versions.

## 2.0.0 (August 15, 2018)

BACKWARDS INCOMPATIBILITIES / NOTES:
* `random_string`: set the ID for random_string resources to "none". Any terraform configuration referring to `random_string.foo.id` will need to be updated to reference `random_string.foo.result` ([#17](https://github.com/terraform-providers/terraform-provider-random/issues/17))

NEW FEATURES:

* `random_uuid` generates random uuid string that is intended to be used as unique identifiers for other resources ([#38](https://github.com/terraform-providers/terraform-provider-random/issues/38))

BUG FIXES: 
* Use UnixNano() instead of Unix() for the current time seed in NewRand() ([#27](https://github.com/terraform-providers/terraform-provider-random/issues/27))
* `random_shuffle`: if `random_shuffle` is given an empty list, it will return an empty list

IMPROVEMENTS:

* Replace ReadPet function in `resource_pet` with schema.Noop ([#34](https://github.com/terraform-providers/terraform-provider-random/issues/34))

## 1.3.1 (May 22, 2018)

BUG FIXES:

* Add migration and new schema version for `resource_string` ([#29](https://github.com/terraform-providers/terraform-provider-random/issues/29))

## 1.3.0 (May 21, 2018)

BUG FIXES:

* `random_integer` now supports update ([#25](https://github.com/terraform-providers/terraform-provider-random/issues/25))

IMPROVEMENTS:

* Add optional minimum character constraints to `random_string` ([#22](https://github.com/terraform-providers/terraform-provider-random/issues/22))

## 1.2.0 (April 03, 2018)

NEW FEATURES:

* `random_integer` and `random_id` are now importable. ([#20](https://github.com/terraform-providers/terraform-provider-random/issues/20))

## 1.1.0 (December 01, 2017)

NEW FEATURES:

* `random_integer` resource generates a single integer within a given range. ([#12](https://github.com/terraform-providers/terraform-provider-random/issues/12))

## 1.0.0 (September 15, 2017)

NEW FEATURES:

* `random_string` resource generates random strings of a given length consisting of letters, digits and symbols. ([#5](https://github.com/terraform-providers/terraform-provider-random/issues/5))

## 0.1.0 (June 21, 2017)

NOTES:

* Same functionality as that of Terraform 0.9.8. Repacked as part of [Provider Splitout](https://www.hashicorp.com/blog/upcoming-provider-changes-in-terraform-0-10/)
