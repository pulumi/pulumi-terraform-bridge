# Contributing

Thank you for investing your time and energy by contributing to our project: please ensure you are familiar
with the [HashiCorp Code of Conduct](https://github.com/hashicorp/.github/blob/master/CODE_OF_CONDUCT.md).

This provider is a HashiCorp **utility provider**, which means any bug fix and feature
has to be considered in the context of the thousands/millions of configurations in which this provider is used.
This is great as your contribution can have a big positive impact, but we have to assess potential negative impact too
(e.g. breaking existing configurations). _Stability over features_.

To provide some safety to the wider provider ecosystem, we strictly follow
[semantic versioning](https://semver.org/) and HashiCorp's own
[versioning specification](https://www.terraform.io/plugin/sdkv2/best-practices/versioning#versioning-specification).
Any changes that could be considered as breaking will only be included as part of a major release.
In case multiple breaking changes need to happen, we will group them in the next upcoming major release.

## Asking Questions

For questions, curiosity, or if still unsure what you are dealing with,
please see the HashiCorp [Terraform Providers Discuss](https://discuss.hashicorp.com/c/terraform-providers/31)
forum.

## Reporting Vulnerabilities

Please disclose security vulnerabilities responsibly by following the
[HashiCorp Vulnerability Reporting guidelines](https://www.hashicorp.com/security#vulnerability-reporting).

## Understanding the design

Before proceeding with raising issues or submitting pull requests, it will probably help to familiarise yourself with
the [design principles](../DESIGN.md) of this provider. This will aid your proposals, and help understand
why we took certain decisions during development.

## Raising Issues

We welcome issues of all kinds including feature requests, bug reports or documentation suggestions.
Below are guidelines for well-formed issues of each type.

### Bug Reports

* [ ] **Test against latest release**: Make sure you test against the latest available version of Terraform and the provider.
  It is possible we may have already fixed the bug you're experiencing.
* [ ] **Search for duplicates**: It's helpful to keep bug reports consolidated to one thread, so do a quick search
  on existing bug reports to check if anybody else has reported the same thing.
  You can scope searches by the label `bug` to help narrow things down.
* [ ] **Include steps to reproduce**: Provide steps to reproduce the issue, along with code examples and/or real code,
  so we can try to reproduce it. Without this, it makes it much harder (sometimes impossible) to fix the issue.

### Feature Requests

* [ ] **Search for possible duplicate requests**: It's helpful to keep requests consolidated to one thread,
  so do a quick search on existing requests to check if anybody else has reported the same thing.
  You can scope searches by the label `enhancement` to help narrow things down.
* [ ] **Include a use case description**: In addition to describing the behavior of the feature you'd like to see added,
  it's helpful to also make a case for why the feature would be important and how it would benefit
  the provider and, potentially, the wider Terraform ecosystem.

## New Pull Request

Thank you for contributing!

We are happy to review pull requests without associated issues,
but we **highly recommend** starting by describing and discussing
your problem or feature and attaching use cases to an issue first
before raising a pull request.

* [ ] **Early validation of idea and implementation plan**: provider development is complicated enough that there
  are often several ways to implement something, each of which has different implications and tradeoffs.
  Working through a plan of attack with the team before you dive into implementation will help ensure that you're
  working in the right direction.
* [ ] **Tests**: It may go without saying, but every new patch should be covered by tests wherever possible.
  For bug-fixes, tests to prove the fix is valid. For features, tests to exercise the new code paths.
* [ ] **Go Modules**: We use [Go Modules](https://github.com/golang/go/wiki/Modules) to manage and version our dependencies.
  Please make sure that you reflect dependency changes in your pull requests appropriately
  (e.g. `go get`, `go mod tidy` or other commands).
  Refer to the [dependency updates](#dependency-updates) section for more information about how
  this project maintains existing dependencies.
* [ ] **Changelog**: Refer to the [changelog](#changelog) section for more information about how to create changelog entries.

### Dependency Updates

Dependency management is performed by [Dependabot](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates).
Where possible, dependency updates should occur through that system to ensure all Go module files are appropriately
updated and to prevent duplicated effort of concurrent update submissions.
Once available, updates are expected to be verified and merged to prevent latent technical debt.

### Changelog

HashiCorp’s open-source projects have always maintained user-friendly, readable `CHANGELOG`s that allow
practitioners and developers to tell at a glance whether a release should have any effect on them,
and to gauge the risk of an upgrade.

We follow Terraform Plugin
[changelog specifications](https://www.terraform.io/plugin/sdkv2/best-practices/versioning#changelog-specification).

#### Entry format

Entries that are specific to _resources_ or _data sources_, they should look like:

```markdown
* resource/RESOURCE_NAME: ENTRY DESCRIPTION ([#PR_NUMBER](PR_URL)).

* data-source/DATA-SOURCE_NAME: ENTRY DESCRIPTION ([#PR_NUMBER](PR_URL)).
```

#### Pull Request Types to `CHANGELOG`

The `CHANGELOG` is intended to show developer-impacting changes to the codebase for a particular version.
If every change or commit to the code resulted in an entry, the `CHANGELOG` would become less useful for developers.
The lists below are general guidelines to decide whether a change should have an entry.

##### Changes that should not have a `CHANGELOG` entry

* Documentation updates
* Testing updates
* Code refactoring

##### Changes that may have a `CHANGELOG` entry

* Dependency updates: If the update contains relevant bug fixes or enhancements that affect developers,
  those should be called out.

##### Changes that should have a `CHANGELOG` entry

* Major features
* Bug fixes
* Enhancements
* Deprecations
* Breaking changes and removals
