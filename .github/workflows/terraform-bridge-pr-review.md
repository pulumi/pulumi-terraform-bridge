---
description: Automated PR review for trusted internal contributors.
timeout-minutes: 15
strict: true
on:
  pull_request:
    types: [opened]
  workflow_dispatch:
    inputs:
      pr_number:
        description: "Pull request number to review"
        required: true
        type: string
imports:
  - pulumi-labs/gh-aw-internal/.github/snippets/code-review.md@main
permissions:
  contents: read
  pull-requests: read
  id-token: write
engine:
  id: claude
  env:
    ANTHROPIC_API_KEY: ${{ steps.esc-secrets.outputs.ANTHROPIC_API_KEY || '__GH_AW_ACTIVATION_PLACEHOLDER__' }}
steps:
  - env:
      ESC_ACTION_ENVIRONMENT: imports/github-secrets
      ESC_ACTION_EXPORT_ENVIRONMENT_VARIABLES: "false"
      ESC_ACTION_OIDC_AUTH: "true"
      ESC_ACTION_OIDC_ORGANIZATION: pulumi
      ESC_ACTION_OIDC_REQUESTED_TOKEN_TYPE: urn:pulumi:token-type:access_token:organization
    id: esc-secrets
    name: Fetch secrets from ESC
    uses: pulumi/esc-action@9eb774255b1a4afb7855678ae8d4a77359da0d9b
  - name: Validate ESC secret output
    env:
      ANTHROPIC_API_KEY_FROM_ESC: ${{ steps.esc-secrets.outputs.ANTHROPIC_API_KEY }}
    run: |
      test -n "$ANTHROPIC_API_KEY_FROM_ESC" || {
        echo "ESC did not return ANTHROPIC_API_KEY";
        exit 1;
      }
tools:
  github:
    lockdown: false
    toolsets: [pull_requests, repos]
safe-outputs:
  create-pull-request-review-comment:
    max: 12
    side: "RIGHT"
    target: "${{ github.event.pull_request.number || github.event.inputs.pr_number }}"
    target-repo: "${{ github.repository }}"
  submit-pull-request-review:
    max: 1
    target: "${{ github.event.pull_request.number || github.event.inputs.pr_number }}"
  noop:
    max: 1
  messages:
    footer: "> Reviewed by [{workflow_name}]({run_url})"
    run-started: "Started automated PR review for #${{ github.event.pull_request.number || github.event.inputs.pr_number }}."
    run-success: "Finished automated PR review for #${{ github.event.pull_request.number || github.event.inputs.pr_number }}."
    run-failure: "Automated PR review failed for #${{ github.event.pull_request.number || github.event.inputs.pr_number }} ({status})."
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-review.md@99a309bf18258dbae98c8176ea10657e0b08deca
---

# Internal Trusted PR Reviewer

Review pull request #${{ github.event.pull_request.number || github.event.inputs.pr_number }} in repository `${{ github.repository }}`.
This workflow imports `pulumi-labs/gh-aw-internal/.github/snippets/code-review.md@main` for the baseline review rubric.

## Trust Model

This workflow supports both `pull_request` and manual `workflow_dispatch` triggers.
For `pull_request`, it uses gh-aw default fork filtering (same-repository PRs only unless `forks` is explicitly configured).
`tools.github.lockdown: false` is set to avoid requiring a custom GitHub MCP token.
If required PR context cannot be read in this trust model, call `noop` with a brief reason and stop.

## Workflow-Specific Rules

- Use `${{ github.event.pull_request.number || github.event.inputs.pr_number }}` as the authoritative PR target.
- Ignore discovery steps intended for runs without PR context.
- Use `create-pull-request-review-comment` for actionable inline findings on changed lines.
- Submit exactly one final review with `submit-pull-request-review`:
  - `REQUEST_CHANGES` when at least one blocking issue exists.
  - `APPROVE` otherwise, including when only non-blocking observations exist.
  - Do not submit `COMMENT` as the final review state.
- If there is nothing to do because trust/context checks fail, call `noop`.

Constraints:
- Post no more than 12 inline comments.
- Do not post free-form issue comments outside review safe outputs.
