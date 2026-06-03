---
on:
  slash_command:
<<<<<<< current (local changes)
    events:
    - pull_request_comment
    - pull_request_review_comment
    name: review-again
||||||| base (original)
    name: review-again
    events: [pull_request_comment, pull_request_review_comment]
imports:
  - shared/review.md
  - shared/plugins/code-review/code-review.md
=======
    name: [review-again, review]
    events: [pull_request_comment, pull_request_review_comment]
imports:
  - shared/review.md
  - shared/plugins/code-review/code-review.md
>>>>>>> new (upstream)
permissions:
  contents: read
  id-token: write
<<<<<<< current (local changes)
  pull-requests: read
imports:
- pulumi-labs/gh-aw-internal/.github/workflows/shared/review.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
- pulumi-labs/gh-aw-internal/.github/workflows/shared/plugins/code-review/code-review.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
description: Run PR re-review on explicit maintainer slash command.
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-rereview.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
safe-outputs:
  threat-detection: false
strict: true
timeout-minutes: 15
||||||| base (original)
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-rereview.md@8a92f53fac170563f7727cacab2dbedb5d5b9e29
=======
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-rereview.md@242988150273951aad5f67b008256266bdff6112
>>>>>>> new (upstream)
---
# Internal PR Re-Review (Slash Command)

Draft review policy: This workflow is an explicit maintainer-requested slash-command review. Review draft PRs; do not call `noop` solely because the pull request is a draft.

Accepted slash commands: `/review-again` and `/review`.
