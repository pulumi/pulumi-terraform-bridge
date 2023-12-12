import argparse
import json
import requests
import subprocess as sp

ap = argparse.ArgumentParser()
ap.add_argument('--hash', required=True)
args = ap.parse_args()
c = args.hash

query = """
{
  search(query: "is:open is:pr org:pulumi ???", type: ISSUE, first: 100) {
    edges {
      node {
        ... on PullRequest {
          number
          url
          mergeable
          title
          commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
        }
      }
    }
  }
}
""".replace("???", c)


token = sp.check_output(["gh", "auth", "token"]).decode('utf-8').strip()

resp = requests.post('https://api.github.com/graphql',
                     headers={'Authorization': f'bearer {token}'},
                     json={"query": query})

r = resp.json()

for e in r['data']['search']['edges']:
    if e.get('node', {}).get('title', '') != f'Upgrade pulumi-terraform-bridge to {c}':
        continue

    checks = e.get('node', {}).get('commits', {}).get('nodes', [{}])[0]['commit']['statusCheckRollup']['state']
    if checks == 'SUCCESS':
        url = e['node']['url']
        sp.check_call(['gh', 'pr', 'close', url])
    else:
        print('FAILED', e['node']['url'])
