from typing import Any, Iterator
import argparse
import subprocess as sp

import requests


QUERY = """
{
  search(query: "is:open is:pr org:pulumi ???", type: ISSUE, first: 100) {
    edges {
      node {
        ... on PullRequest {
          number
          url
          mergeable
          title
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


def get_title(query_result: Any) -> str:
    return str(query_result.get("node", {}).get("title", ""))


def iterate_prs(query_result: Any) -> Iterator[tuple[str, list[Any]]]:
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
        yield title, checks, url


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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--hash", required=True)
    args = ap.parse_args()
    c = args.hash

    replaced_query = QUERY.replace("???", c)
    token = sp.check_output(["gh", "auth", "token"]).decode("utf-8").strip()
    r = get_gh_data(token, replaced_query)

    pr_suffix = f"Upgrade pulumi-terraform-bridge to {c}"

    for pr_title, pr_checks, url in iterate_prs(r):
        if not pr_title.endswith(pr_suffix):
            continue

        sentinel_status = get_sentinel_status(pr_checks)
        if sentinel_status == "SUCCESS":
            print("SUCCESS", url)
            sp.check_call(["gh", "pr", "close", url])
        else:
            print(sentinel_status, url)


if __name__ == "__main__":
    main()
