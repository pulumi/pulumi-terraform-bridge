#!/usr/bin/env bash

set -xeuo pipefail

# This script will stand up example PRs in the target provider

usage() {
    echo "compare-docs.sh - Create a PR in a provider repo to compare docs changes"
    echo
    echo "compare-docs.sh <repo> "
    echo
    echo "repo - the path to the repo you are testing against."
    echo "       For example: ../pulumi-aws"
}

UPSTREAM_REPO="$1"

if [ "$UPSTREAM_REPO" = "" ]; then
    echo "missing <repo>"
    echo
    usage
    exit 1
fi

UPSTREAM_REPO=$(realpath "$UPSTREAM_REPO")

# Gather information about the current PR in pulumi-terraform-bridge:

# The name of the current branch
BRANCH=$(git symbolic-ref HEAD)
BRANCH=${BRANCH#refs/heads/} # strip refs/heads

# SHA at origin/master
ORIGIN_SHA=$(git ls-remote --heads | grep refs/heads/master | awk '{print $1}')

# SHA at the head of origin/${BRANCH}
HEAD_SHA=$(git ls-remote --heads | grep "$BRANCH" | awk '{print $1}')

# Info about the current PR for display purposes
PR_INFO="$(gh pr view "$BRANCH" --json url,title,number)"
PR_NAME="$(echo "$PR_INFO" | jq -r .title) #$(echo "$PR_INFO" | jq .number)"
PR_URL="$(echo "$PR_INFO" | jq -r .url)"

cd "$UPSTREAM_REPO"
DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')

git checkout -b docs-example "$DEFAULT_BRANCH" || git checkout docs-example
git fetch
git reset --hard "origin/$DEFAULT_BRANCH"

replace_bridge() {
    local cwd="$PWD"
    cd "$UPSTREAM_REPO/provider"
    go mod edit \
       -replace \
       "github.com/pulumi/pulumi-terraform-bridge/v3=github.com/pulumi/pulumi-terraform-bridge/v3@$1" \
       -replace \
       "github.com/pulumi/pulumi-terraform-bridge/pf=github.com/pulumi/pulumi-terraform-bridge/pf@$1" \
       -replace \
       "github.com/pulumi/pulumi-terraform-bridge/x/muxer=github.com/pulumi/pulumi-terraform-bridge/x/muxer@$1" \
       -fmt
    go mod tidy
    cd "$cwd"
}

tfgen() {
    make tfgen
    git add provider/go.*
    git add provider/cmd/pulumi-resource-*/schema.json
}

# Set the first commit to show master.
#
# This is useful when multiple docs changes come between releases.
replace_bridge "$ORIGIN_SHA"
tfgen
git commit -m "Schema generation for bridge master"

# Set the second commit to show the difference between master and the current branch.
#
# In general, this is the commit that we are interested in.
replace_bridge "$HEAD_SHA"
tfgen
git commit -m "Schema generation for $BRANCH - $HEAD_SHA"

# We try to push a new branch
if git push --set-upstream origin docs-example; then
    # If we succeed, this is a new branch and we should open an associated PR to better
    # display the diff.
    gh pr create \
       --assignee @me \
       --draft \
       --title "Show changes from $PR_NAME" \
       --body "$(
    echo "## DO NOT MERGE"
    echo
    echo "This PR was created to demonstrate the effects of [$PR_NAME]($PR_URL)."
    )"
else
    # If we failed we try a force push. If this succeeds then there is probably a PR
    # already from a previous invocation of this script.
    git push --force --set-upstream origin docs-example
fi
