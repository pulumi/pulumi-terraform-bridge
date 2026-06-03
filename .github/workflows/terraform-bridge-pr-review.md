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
<<<<<<< current (local changes)
- pulumi-labs/gh-aw-internal/.github/workflows/shared/review.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
- pulumi-labs/gh-aw-internal/.github/workflows/shared/plugins/code-review/code-review.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
description: Automated PR review for trusted internal contributors.
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-review.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
safe-outputs:
  threat-detection: false
strict: true
timeout-minutes: 15
||||||| base (original)
  - shared/review.md
  - shared/plugins/code-review/code-review.md
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-review.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
=======
  - shared/review.md
  - shared/plugins/code-review/code-review.md
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-review.md@242988150273951aad5f67b008256266bdff6112
>>>>>>> new (upstream)
---
# Internal Trusted PR Reviewer

Draft review policy: This workflow may review draft PRs only when manually dispatched with `workflow_dispatch`. For automatic `pull_request` runs, call `noop` if the pull request is a draft.
