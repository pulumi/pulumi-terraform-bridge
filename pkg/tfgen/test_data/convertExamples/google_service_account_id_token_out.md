# google_service_account_id_token

This data source provides a Google OpenID Connect (`oidc`) `id_token`.  Tokens issued from this data source are typically used to call external services that accept OIDC tokens for authentication (e.g. [Google Cloud Run](https://cloud.google.com/run/docs/authenticating/service-to-service)).

For more information see
[OpenID Connect](https://openid.net/specs/openid-connect-core-1_0.html#IDToken).

## Example Usage - Service Account Impersonation.
`google_service_account_id_token` will use background impersonated credentials provided by [google_service_account_access_token](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/service_account_access_token).

Note: to use the following, you must grant `target_service_account` the
`roles/iam.serviceAccountTokenCreator` role on itself.

  <!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const impersonatedAccountAccessToken = gcp.serviceaccount.getAccountAccessToken({
    targetServiceAccount: "impersonated-account@project.iam.gserviceaccount.com",
    delegates: [],
    scopes: [
        "userinfo-email",
        "cloud-platform",
    ],
    lifetime: "300s",
});
const impersonated = new pulumi.providers.Google("impersonated", {accessToken: impersonatedAccountAccessToken.accessToken});
const oidc = gcp.serviceaccount.getAccountIdToken({
    targetServiceAccount: "impersonated-account@project.iam.gserviceaccount.com",
    delegates: [],
    includeEmail: true,
    targetAudience: "https://foo.bar/",
});
export const oidcToken = oidc.then(oidc => oidc.idToken);
```
```python
import pulumi
import pulumi_gcp as gcp

impersonated_account_access_token = gcp.serviceaccount.get_account_access_token(target_service_account="impersonated-account@project.iam.gserviceaccount.com",
    delegates=[],
    scopes=[
        "userinfo-email",
        "cloud-platform",
    ],
    lifetime="300s")
impersonated = pulumi.providers.Google("impersonated", access_token=impersonated_account_access_token.access_token)
oidc = gcp.serviceaccount.get_account_id_token(target_service_account="impersonated-account@project.iam.gserviceaccount.com",
    delegates=[],
    include_email=True,
    target_audience="https://foo.bar/")
pulumi.export("oidcToken", oidc.id_token)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Gcp = Pulumi.Gcp;

return await Deployment.RunAsync(() => 
{
    var impersonatedAccountAccessToken = Gcp.ServiceAccount.GetAccountAccessToken.Invoke(new()
    {
        TargetServiceAccount = "impersonated-account@project.iam.gserviceaccount.com",
        Delegates = new() { },
        Scopes = new[]
        {
            "userinfo-email",
            "cloud-platform",
        },
        Lifetime = "300s",
    });

    var impersonated = new Pulumi.Providers.Google("impersonated", new()
    {
        AccessToken = impersonatedAccountAccessToken.Apply(getAccountAccessTokenResult => getAccountAccessTokenResult.AccessToken),
    });

    var oidc = Gcp.ServiceAccount.GetAccountIdToken.Invoke(new()
    {
        TargetServiceAccount = "impersonated-account@project.iam.gserviceaccount.com",
        Delegates = new() { },
        IncludeEmail = true,
        TargetAudience = "https://foo.bar/",
    });

    return new Dictionary<string, object?>
    {
        ["oidcToken"] = oidc.Apply(getAccountIdTokenResult => getAccountIdTokenResult.IdToken),
    };
});
```
```go
package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-google/sdk/go/google"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		impersonatedAccountAccessToken, err := serviceaccount.GetAccountAccessToken(ctx, &serviceaccount.GetAccountAccessTokenArgs{
			TargetServiceAccount: "impersonated-account@project.iam.gserviceaccount.com",
			Delegates:            []interface{}{},
			Scopes: []string{
				"userinfo-email",
				"cloud-platform",
			},
			Lifetime: pulumi.StringRef("300s"),
		}, nil)
		if err != nil {
			return err
		}
		_, err = google.NewProvider(ctx, "impersonated", &google.ProviderArgs{
			AccessToken: impersonatedAccountAccessToken.AccessToken,
		})
		if err != nil {
			return err
		}
		oidc, err := serviceaccount.GetAccountIdToken(ctx, &serviceaccount.GetAccountIdTokenArgs{
			TargetServiceAccount: pulumi.StringRef("impersonated-account@project.iam.gserviceaccount.com"),
			Delegates:            []interface{}{},
			IncludeEmail:         pulumi.BoolRef(true),
			TargetAudience:       "https://foo.bar/",
		}, nil)
		if err != nil {
			return err
		}
		ctx.Export("oidcToken", oidc.IdToken)
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.gcp.serviceaccount.ServiceaccountFunctions;
import com.pulumi.gcp.serviceaccount.inputs.GetAccountAccessTokenArgs;
import com.pulumi.pulumi.providers.google;
import com.pulumi.pulumi.providers.ProviderArgs;
import com.pulumi.gcp.serviceaccount.inputs.GetAccountIdTokenArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        final var impersonatedAccountAccessToken = ServiceaccountFunctions.getAccountAccessToken(GetAccountAccessTokenArgs.builder()
            .targetServiceAccount("impersonated-account@project.iam.gserviceaccount.com")
            .delegates()
            .scopes(            
                "userinfo-email",
                "cloud-platform")
            .lifetime("300s")
            .build());

        var impersonated = new Provider("impersonated", ProviderArgs.builder()
            .accessToken(impersonatedAccountAccessToken.applyValue(getAccountAccessTokenResult -> getAccountAccessTokenResult.accessToken()))
            .build());

        final var oidc = ServiceaccountFunctions.getAccountIdToken(GetAccountIdTokenArgs.builder()
            .targetServiceAccount("impersonated-account@project.iam.gserviceaccount.com")
            .delegates()
            .includeEmail(true)
            .targetAudience("https://foo.bar/")
            .build());

        ctx.export("oidcToken", oidc.applyValue(getAccountIdTokenResult -> getAccountIdTokenResult.idToken()));
    }
}
```
```yaml
resources:
  impersonated:
    type: pulumi:providers:google
    properties:
      accessToken: ${impersonatedAccountAccessToken.accessToken}
variables:
  impersonatedAccountAccessToken:
    fn::invoke:
      function: gcp:serviceaccount:getAccountAccessToken
      arguments:
        targetServiceAccount: impersonated-account@project.iam.gserviceaccount.com
        delegates: []
        scopes:
          - userinfo-email
          - cloud-platform
        lifetime: 300s
  oidc:
    fn::invoke:
      function: gcp:serviceaccount:getAccountIdToken
      arguments:
        targetServiceAccount: impersonated-account@project.iam.gserviceaccount.com
        delegates: []
        includeEmail: true
        targetAudience: https://foo.bar/
outputs:
  oidcToken: ${oidc.idToken}
```
<!--End PulumiCodeChooser -->

## Example Usage - Invoking Cloud Run Endpoint

The following configuration will invoke [Cloud Run](https://cloud.google.com/run/docs/authenticating/service-to-service) endpoint where the service account for Terraform has been granted `roles/run.invoker` role previously.

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import * as http from "@pulumi/http";

const oidc = gcp.serviceaccount.getAccountIdToken({
    targetAudience: "https://your.cloud.run.app/",
});
const cloudrun = oidc.then(oidc => http.getHttp({
    url: "https://your.cloud.run.app/",
    requestHeaders: {
        Authorization: `Bearer ${oidc.idToken}`,
    },
}));
export const cloudRunResponse = cloudrun.then(cloudrun => cloudrun.body);
```
```python
import pulumi
import pulumi_gcp as gcp
import pulumi_http as http

oidc = gcp.serviceaccount.get_account_id_token(target_audience="https://your.cloud.run.app/")
cloudrun = http.get_http(url="https://your.cloud.run.app/",
    request_headers={
        "Authorization": f"Bearer {oidc.id_token}",
    })
pulumi.export("cloudRunResponse", cloudrun.body)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Gcp = Pulumi.Gcp;
using Http = Pulumi.Http;

return await Deployment.RunAsync(() => 
{
    var oidc = Gcp.ServiceAccount.GetAccountIdToken.Invoke(new()
    {
        TargetAudience = "https://your.cloud.run.app/",
    });

    var cloudrun = Http.GetHttp.Invoke(new()
    {
        Url = "https://your.cloud.run.app/",
        RequestHeaders = 
        {
            { "Authorization", $"Bearer {oidc.Apply(getAccountIdTokenResult => getAccountIdTokenResult.IdToken)}" },
        },
    });

    return new Dictionary<string, object?>
    {
        ["cloudRunResponse"] = cloudrun.Apply(getHttpResult => getHttpResult.Body),
    };
});
```
```go
package main

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-http/sdk/go/http"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		oidc, err := serviceaccount.GetAccountIdToken(ctx, &serviceaccount.GetAccountIdTokenArgs{
			TargetAudience: "https://your.cloud.run.app/",
		}, nil)
		if err != nil {
			return err
		}
		cloudrun, err := http.GetHttp(ctx, &http.GetHttpArgs{
			Url: "https://your.cloud.run.app/",
			RequestHeaders: map[string]interface{}{
				"Authorization": fmt.Sprintf("Bearer %v", oidc.IdToken),
			},
		}, nil)
		if err != nil {
			return err
		}
		ctx.Export("cloudRunResponse", cloudrun.Body)
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.gcp.serviceaccount.ServiceaccountFunctions;
import com.pulumi.gcp.serviceaccount.inputs.GetAccountIdTokenArgs;
import com.pulumi.http.HttpFunctions;
import com.pulumi.http.inputs.GetHttpArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        final var oidc = ServiceaccountFunctions.getAccountIdToken(GetAccountIdTokenArgs.builder()
            .targetAudience("https://your.cloud.run.app/")
            .build());

        final var cloudrun = HttpFunctions.getHttp(GetHttpArgs.builder()
            .url("https://your.cloud.run.app/")
            .requestHeaders(Map.of("Authorization", String.format("Bearer %s", oidc.applyValue(getAccountIdTokenResult -> getAccountIdTokenResult.idToken()))))
            .build());

        ctx.export("cloudRunResponse", cloudrun.applyValue(getHttpResult -> getHttpResult.body()));
    }
}
```
```yaml
variables:
  oidc:
    fn::invoke:
      function: gcp:serviceaccount:getAccountIdToken
      arguments:
        targetAudience: https://your.cloud.run.app/
  cloudrun:
    fn::invoke:
      function: http:getHttp
      arguments:
        url: https://your.cloud.run.app/
        requestHeaders:
          Authorization: Bearer ${oidc.idToken}
outputs:
  cloudRunResponse: ${cloudrun.body}
```
<!--End PulumiCodeChooser -->
