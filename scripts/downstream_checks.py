from typing import Any, Iterator
import argparse
import subprocess as sp
import requests

NOT_BRIDGED_PROVIDERS = [
    "aws-apigateway",
    "awsx",
    "eks",
    "terraform-module",
]

QUERY = """
{
  search(query: "is:pr org:pulumi ??? in:title", type: ISSUE, first: 100) {
    edges {
      node {
        ... on PullRequest {
          number
          url
          mergeable
          title
          closed
          repository {
            name
          }
          commits(last: 1) {
            nodes {
              commit {
                statusCheckRollup {
                  contexts(last: 100) {
                    nodes {
                      ... on CheckRun {
                        status
                        name
                        conclusion
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
"""


def get_provider_map() -> dict[str, bool]:
    resp = requests.get(
        "https://raw.githubusercontent.com/pulumi/ci-mgmt/master/provider-ci/providers.json"
    )
    return {k: False for k in resp.json() if k not in NOT_BRIDGED_PROVIDERS}


def get_title(query_result: Any) -> str:
    return str(query_result.get("node", {}).get("title", ""))


def get_repo_name(query_result: Any) -> str:
    return str(query_result.get("node", {}).get("repository", {}).get("name", ""))


def iterate_prs(query_result: Any) -> Iterator[tuple[str, bool, list[Any], str, str]]:
    for e in query_result.get("data", {}).get("search", {}).get("edges", {}):
        title = get_title(e)
        checks = (
            e.get("node", {})
            .get("commits", {})
            .get("nodes", [{}])[0]["commit"]["statusCheckRollup"]
            .get("contexts", {})
            .get("nodes", [{}])
        )
        url = e["node"]["url"]
        repo = get_repo_name(e)
        closed = e["node"]["closed"]
        yield title, closed, checks, url, repo


def get_sentinel_status(checks: list[Any]) -> str:
    for check in checks:
        if not check:
            # Skip empty checks
            continue
        if check["name"] == "sentinel":
            if check["status"] != "COMPLETED":
                return check["status"]
            return check["conclusion"]
    return "UNKNOWN"


def get_gh_data(token: str, query: str) -> dict[str, Any]:
    resp = requests.post(
        "https://api.github.com/graphql",
        headers={"Authorization": f"bearer {token}"},
        json={"query": query},
        timeout=300,  # seconds
    )
    resp.raise_for_status()
    return resp.json()


def repo_actions_url(repo: str) -> str:
    return "https://github.com/pulumi/pulumi-" + repo + "/actions/workflows/upgrade-bridge.yml"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--hash", required=True)
    ap.add_argument("--show-closed", action="store_true")
    ap.add_argument("--only-failed", action="store_true")
    args = ap.parse_args()
    c = args.hash
    show_closed: bool = args.show_closed
    only_failed: bool = args.only_failed

    replaced_query = QUERY.replace("???", c)
    token = sp.check_output(["gh", "auth", "token"]).decode("utf-8").strip()
    r = get_gh_data(token, replaced_query)

    pr_suffix = f"Upgrade pulumi-terraform-bridge to {c}"

    provider_map = get_provider_map()

    for pr_title, closed, pr_checks, url, repo in iterate_prs(r):
        if not pr_title.endswith(pr_suffix):
            continue

        provider_map[repo.removeprefix("pulumi-")] = True

        sentinel_status = get_sentinel_status(pr_checks)

        if sentinel_status == "SUCCESS":
            if not only_failed:
                print("SUCCESS", url)
            if not closed:
                sp.check_call(["gh", "pr", "close", url])
        else:
            if not closed or show_closed:
                # If the sentinel is SKIPPED, this very likely means the PR failed some checks.
                if sentinel_status == "SKIPPED":
                    print('FAILED:', url)
                else:
                    print(sentinel_status, url)

    for missing_repo in {repo for repo in provider_map if not provider_map[repo]}:
        # Possibly waiting for plumbing to propagate, we do not know for sure.
        print("WAITING", missing_repo)


if __name__ == "__main__":
    main()
