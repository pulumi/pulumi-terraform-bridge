---
description: Run PR re-review on explicit maintainer slash command.
timeout-minutes: 15
strict: true
on:
  slash_command:
    name: review-again
    events: [pull_request_comment, pull_request_review_comment]
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
    target: "${{ github.event.pull_request.number || github.event.issue.number }}"
    target-repo: "${{ github.repository }}"
  submit-pull-request-review:
    max: 1
    target: "${{ github.event.pull_request.number || github.event.issue.number }}"
  noop:
    max: 1
  messages:
    footer: "> Reviewed by [{workflow_name}]({run_url})"
    run-started: "Started slash-command PR re-review."
    run-success: "Finished slash-command PR re-review."
    run-failure: "Slash-command PR re-review failed ({status})."
source: pulumi-labs/gh-aw-internal/.github/workflows/gh-aw-pr-rereview.md@99a309bf18258dbae98c8176ea10657e0b08deca
---

# Internal PR Re-Review (Slash Command)

This workflow runs when a maintainer posts `/review-again` as the first token in a PR comment or review comment.

Review context text: `${{ needs.activation.outputs.text }}`

This workflow imports `pulumi-labs/gh-aw-internal/.github/snippets/code-review.md@main` for the baseline review rubric.

## Trust Model

- This workflow is slash-command triggered and restricted to PR comment/review-comment events.
- `tools.github.lockdown: false` is set to avoid requiring a custom GitHub MCP token.
- If the command was not issued in PR context, call `noop` and stop.

## Workflow-Specific Rules

- Determine the target PR number from the command event context.
- Use that PR number for all review operations in this run.
- Use `create-pull-request-review-comment` for actionable inline findings on changed lines.
- Submit exactly one final review with `submit-pull-request-review`:
  - `REQUEST_CHANGES` when at least one blocking issue exists.
  - `APPROVE` otherwise, including when only non-blocking observations exist.
  - Do not submit `COMMENT` as the final review state.
- If required PR context is missing, call `noop`.

Constraints:
- Post no more than 12 inline comments.
- Do not post free-form issue comments outside review safe outputs.
