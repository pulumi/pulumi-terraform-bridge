The [Artifactory](https://jfrog.com/artifactory/) provider is used to interact with the resources supported by Artifactory. The provider needs to be configured with the proper credentials before it can be used.

Links to documentation for specific resources can be found in the table of contents to the left.

This provider requires access to Artifactory APIs, which are only available in the _licensed_ pro and enterprise editions. You can determine which license you have by accessing the following the URL `${host}/artifactory/api/system/licenses/`.

You can either access it via API, or web browser - it require admin level credentials.

```sh
curl -sL ${host}/artifactory/api/system/licenses/ | jq .
{
  "type" : "Enterprise Plus Trial",
  "validThrough" : "Jan 29, 2022",
  "licensedTo" : "JFrog Ltd"
}
```