---
on:
  pull_request:
    types:
    - opened
    - ready_for_review
  workflow_dispatch:
    inputs:
      pr_number:
        description: Pull request number to review
        required: true
        type: string
permissions:
  contents: read
  id-token: write
  pull-requests: read
imports:
- pulumi-labs/gh-aw-internal/.github/workflows/shared/review.md@242988150273951aad5f67b008256266bdff6112
- pulumi-labs/gh-aw-internal/.github/workflows/shared/plugins/code-review/code-review.md@242988150273951aad5f67b008256266bdff6112
description: Automated PR review for trusted internal contributors.
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-review.md@242988150273951aad5f67b008256266bdff6112
safe-outputs:
  threat-detection: false
strict: true
timeout-minutes: 15
---
# Internal Trusted PR Reviewer

Draft review policy: This workflow may review draft PRs only when manually dispatched with `workflow_dispatch`. For automatic `pull_request` runs, call `noop` if the pull request is a draft.
